package formf

import (
	"reflect"
	"strings"
	"testing"
	"time"
)

// recordOn returns a complete entry with a given serial and procedure date,
// with the surrounding dates moved to match so that nothing else complains.
func recordOn(serial int, date string) Record {
	r := completeRecord()
	r.Serial = serial
	r.ProcedureDate = date
	r.FirstReportedOn = date
	r.WomanDeclarationDate = date
	r.SignedOn = date
	r.DoctorDeclarationDate = date
	r.ConveyedOn = date
	if d, err := ParseDay(date); err == nil {
		r.LMP = FormatDay(d.AddDate(0, 0, -141))
	}
	return r
}

func month(t *testing.T, s string) Month {
	t.Helper()
	m, err := ParseMonth(s)
	if err != nil {
		t.Fatalf("fixture month %q: %v", s, err)
	}
	return m
}

func TestReturnDueDate(t *testing.T) {
	cases := []struct {
		month string
		day   int
		want  string
	}{
		{"2026-11", 5, "2026-12-05"},
		{"2026-12", 5, "2027-01-05"},
		{"2026-01", 5, "2026-02-05"},
		{"2024-01", 5, "2024-02-05"},
		{"2026-02", 5, "2026-03-05"},
		{"2026-11", 10, "2026-12-10"},
		{"2026-01", 28, "2026-02-28"},
	}
	for _, c := range cases {
		t.Run(c.month, func(t *testing.T) {
			s := DefaultSettings()
			s.ReturnDueDay = c.day
			got := FormatDay(ReturnDueDate(month(t, c.month), s))
			if got != c.want {
				t.Errorf("ReturnDueDate(%s, day %d) = %s, want %s", c.month, c.day, got, c.want)
			}
		})
	}
}

// TestEntriesForMonthTakesExactlyTheMonth is the claim that the report is
// assembled from exactly the entries dated in the month — nothing from either
// side of it, nothing undated swept in, and always in serial order.
func TestEntriesForMonthTakesExactlyTheMonth(t *testing.T) {
	records := []Record{
		recordOn(5, "2026-11-03"),
		recordOn(3, "2026-10-31"),  // the day before
		recordOn(7, "2026-11-30"),  // the last day
		recordOn(8, "2026-12-01"),  // the day after
		recordOn(6, "2026-11-01"),  // the first day
		recordOn(9, ""),            // no date at all
		recordOn(10, "20-11-2026"), // not a date
	}
	got := EntriesForMonth(records, month(t, "2026-11"))

	var serials []int
	for _, r := range got {
		serials = append(serials, r.Serial)
	}
	if want := []int{5, 6, 7}; !reflect.DeepEqual(serials, want) {
		t.Errorf("EntriesForMonth returned serials %v, want %v in serial order", serials, want)
	}

	if n := len(EntriesForMonth(records, Month{})); n != 0 {
		t.Errorf("EntriesForMonth with no month returned %d entries", n)
	}
	if n := len(EntriesForMonth(nil, month(t, "2026-11"))); n != 0 {
		t.Errorf("EntriesForMonth of an empty register returned %d entries", n)
	}
}

func TestCheckMonthReconcilesTheCentresOwnCount(t *testing.T) {
	m := month(t, "2026-11")
	cases := []struct {
		name          string
		records       []Record
		tallies       []ProcedureTally
		wantClear     bool
		wantMissing   int
		wantUntallied int
		wantStatuses  map[string]DayStatus
		wantFindingIn []string
	}{
		{
			name:         "the counts agree",
			records:      []Record{recordOn(1, "2026-11-03"), recordOn(2, "2026-11-04")},
			tallies:      []ProcedureTally{{Date: "2026-11-03", Count: 1, Source: "OPD register"}, {Date: "2026-11-04", Count: 1, Source: "OPD register"}},
			wantClear:    true,
			wantStatuses: map[string]DayStatus{"2026-11-03": DayMatched, "2026-11-04": DayMatched},
		},
		{
			name:          "a Form F that was never written",
			records:       []Record{recordOn(1, "2026-11-03")},
			tallies:       []ProcedureTally{{Date: "2026-11-03", Count: 3, Source: "OPD register"}},
			wantMissing:   2,
			wantStatuses:  map[string]DayStatus{"2026-11-03": DayFormsMissing},
			wantFindingIn: []string{"2 procedure(s) carried out in November 2026 have no Form F", "sealing the machine"},
		},
		{
			name:          "a whole day of scans with no entries at all",
			records:       nil,
			tallies:       []ProcedureTally{{Date: "2026-11-14", Count: 4, Source: "machine log"}},
			wantMissing:   4,
			wantStatuses:  map[string]DayStatus{"2026-11-14": DayFormsMissing},
			wantFindingIn: []string{"4 procedure(s)"},
		},
		{
			name:          "entries on a day nobody counted",
			records:       []Record{recordOn(1, "2026-11-07"), recordOn(2, "2026-11-07")},
			tallies:       nil,
			wantUntallied: 2,
			wantStatuses:  map[string]DayStatus{"2026-11-07": DayUntallied},
			wantFindingIn: []string{"no day tally", "unverified"},
		},
		{
			name:          "more entries than the centre counted",
			records:       []Record{recordOn(1, "2026-11-09"), recordOn(2, "2026-11-09")},
			tallies:       []ProcedureTally{{Date: "2026-11-09", Count: 1, Source: "OPD register"}},
			wantStatuses:  map[string]DayStatus{"2026-11-09": DayOverTallied},
			wantFindingIn: []string{},
		},
		{
			name:         "two tally entries for the same day are added together",
			records:      []Record{recordOn(1, "2026-11-11"), recordOn(2, "2026-11-11")},
			tallies:      []ProcedureTally{{Date: "2026-11-11", Count: 1, Source: "morning"}, {Date: "2026-11-11", Count: 1, Source: "evening"}},
			wantClear:    true,
			wantStatuses: map[string]DayStatus{"2026-11-11": DayMatched},
		},
		{
			name:         "a day explicitly counted as nil",
			records:      nil,
			tallies:      []ProcedureTally{{Date: "2026-11-16", Count: 0, Source: "OPD register — Sunday, closed"}},
			wantClear:    true,
			wantStatuses: map[string]DayStatus{"2026-11-16": DayMatched},
		},
		{
			name:          "nothing recorded for the month at all",
			records:       nil,
			tallies:       nil,
			wantFindingIn: []string{"Nothing at all is recorded for November 2026", "only one of them is a record"},
		},
		{
			name:         "tallies from another month are ignored",
			records:      []Record{recordOn(1, "2026-11-03")},
			tallies:      []ProcedureTally{{Date: "2026-11-03", Count: 1}, {Date: "2026-10-03", Count: 9}, {Date: "not a date", Count: 4}},
			wantClear:    true,
			wantStatuses: map[string]DayStatus{"2026-11-03": DayMatched},
		},
		{
			name:          "an incomplete entry inside an otherwise reconciled month",
			records:       []Record{recordOn(1, "2026-11-03"), func() Record { r := recordOn(2, "2026-11-04"); r.Place = ""; return r }()},
			tallies:       []ProcedureTally{{Date: "2026-11-03", Count: 1}, {Date: "2026-11-04", Count: 1}},
			wantStatuses:  map[string]DayStatus{"2026-11-03": DayMatched, "2026-11-04": DayMatched},
			wantFindingIn: []string{"serial 2", "treated as a Form F not maintained"},
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := CheckMonth(c.records, c.tallies, m, DefaultSettings())

			if got.Clear != c.wantClear {
				t.Errorf("Clear = %v, want %v; findings: %v", got.Clear, c.wantClear, got.Findings)
			}
			if got.FormsMissing != c.wantMissing {
				t.Errorf("FormsMissing = %d, want %d", got.FormsMissing, c.wantMissing)
			}
			if got.Untallied != c.wantUntallied {
				t.Errorf("Untallied = %d, want %d", got.Untallied, c.wantUntallied)
			}
			if got.Month != "2026-11" || got.MonthLabel != "November 2026" {
				t.Errorf("month reported as %q / %q", got.Month, got.MonthLabel)
			}

			byDate := map[string]DayReconciliation{}
			for _, d := range got.Days {
				byDate[d.Date] = d
			}
			if len(byDate) != len(got.Days) {
				t.Error("a date appears twice in the day-by-day reconciliation")
			}
			if len(got.Days) != len(c.wantStatuses) {
				t.Errorf("%d day(s) reported, want %d: %+v", len(got.Days), len(c.wantStatuses), got.Days)
			}
			for date, want := range c.wantStatuses {
				d, ok := byDate[date]
				if !ok {
					t.Errorf("no reconciliation reported for %s", date)
					continue
				}
				if d.Status != want {
					t.Errorf("%s: status = %q, want %q (%s)", date, d.Status, want, d.Message)
				}
				if d.Status != DayMatched && d.Message == "" {
					t.Errorf("%s: status %q carries no message", date, d.Status)
				}
				if d.Weekday == "" {
					t.Errorf("%s: no weekday", date)
				}
			}
			all := strings.Join(got.Findings, "\n")
			for _, want := range c.wantFindingIn {
				if !strings.Contains(all, want) {
					t.Errorf("no finding contains %q; findings were:\n%s", want, all)
				}
			}
		})
	}
}

func TestCheckMonthReportsSerialProblemsLoudly(t *testing.T) {
	m := month(t, "2026-11")
	records := []Record{recordOn(1, "2026-11-03"), recordOn(4, "2026-11-04")}
	tallies := []ProcedureTally{{Date: "2026-11-03", Count: 1}, {Date: "2026-11-04", Count: 1}}

	got := CheckMonth(records, tallies, m, DefaultSettings())
	if got.Clear {
		t.Fatal("a month over a register with a gap in its numbering reported itself clear")
	}
	if got.Serial.Intact {
		t.Error("the serial check inside the month reported the numbering intact")
	}
	found := false
	for _, f := range got.Findings {
		if strings.HasPrefix(f, "Numbering: ") && strings.Contains(f, "Serials 2 to 3 are missing") {
			found = true
		}
	}
	if !found {
		t.Errorf("the numbering gap did not reach the month's findings: %v", got.Findings)
	}
}

func TestCheckMonthWithNoMonth(t *testing.T) {
	got := CheckMonth(nil, nil, Month{}, DefaultSettings())
	if got.Clear {
		t.Error("a check with no month reported itself clear")
	}
	if len(got.Findings) == 0 {
		t.Error("a check with no month said nothing")
	}
}

func TestAssembleReturn(t *testing.T) {
	m := month(t, "2026-11")
	asOf := Day(2026, time.December, 2)

	good := []Record{recordOn(1, "2026-11-03"), recordOn(2, "2026-11-04"), recordOn(3, "2026-11-04")}
	tallies := []ProcedureTally{
		{Date: "2026-11-03", Count: 1, Source: "OPD register"},
		{Date: "2026-11-04", Count: 2, Source: "OPD register"},
	}

	t.Run("a month that is ready to send", func(t *testing.T) {
		got := AssembleReturn(testClinic(), good, tallies, m, testSettings(), asOf)

		if !got.Fileable {
			t.Errorf("Fileable = false; blockers: %v", got.Blockers)
		}
		if len(got.Blockers) != 0 {
			t.Errorf("blockers on a clean month: %v", got.Blockers)
		}
		if got.DueOn != "2026-12-05" {
			t.Errorf("DueOn = %q, want 2026-12-05", got.DueOn)
		}
		if got.DaysToDue != 3 {
			t.Errorf("DaysToDue = %d, want 3", got.DaysToDue)
		}
		if got.Overdue {
			t.Error("Overdue = true three days before the due date")
		}
		if got.Totals.Entries != 3 {
			t.Errorf("Totals.Entries = %d, want 3", got.Totals.Entries)
		}
		if got.Totals.Ultrasound != 3 {
			t.Errorf("Totals.Ultrasound = %d, want 3", got.Totals.Ultrasound)
		}
		if got.Totals.SerialFirst != 1 || got.Totals.SerialLast != 3 {
			t.Errorf("serial range %d to %d, want 1 to 3", got.Totals.SerialFirst, got.Totals.SerialLast)
		}
		if got.Totals.FirstDate != "2026-11-03" || got.Totals.LastDate != "2026-11-04" {
			t.Errorf("dates %s to %s, want 2026-11-03 to 2026-11-04", got.Totals.FirstDate, got.Totals.LastDate)
		}
		if got.Totals.DistinctDays != 2 {
			t.Errorf("DistinctDays = %d, want 2", got.Totals.DistinctDays)
		}
		if len(got.Entries) != 3 {
			t.Errorf("the report carries %d entries, want 3", len(got.Entries))
		}
		if !strings.Contains(got.Reference, "rule 9(8)") {
			t.Errorf("Reference = %q", got.Reference)
		}
		if !strings.Contains(got.FormatNote, "prescribes no format") {
			t.Errorf("FormatNote = %q", got.FormatNote)
		}
		// The totals are stable between runs, which matters because a
		// clinic prints the same month twice.
		again := AssembleReturn(testClinic(), good, tallies, m, testSettings(), asOf)
		if !reflect.DeepEqual(got.Totals, again.Totals) {
			t.Error("two runs of the same month produced different totals")
		}
	})

	t.Run("overdue", func(t *testing.T) {
		got := AssembleReturn(testClinic(), good, tallies, m, testSettings(), Day(2026, time.December, 20))
		if !got.Overdue {
			t.Error("Overdue = false on 20 December for a November report due on the 5th")
		}
		if got.DaysToDue != -15 {
			t.Errorf("DaysToDue = %d, want -15", got.DaysToDue)
		}
	})

	t.Run("blocked for every reason at once", func(t *testing.T) {
		records := []Record{
			recordOn(1, "2026-11-03"),
			func() Record { r := recordOn(5, "2026-11-04"); r.Place = ""; return r }(),
			recordOn(6, "2026-11-20"),
		}
		shortTallies := []ProcedureTally{
			{Date: "2026-11-03", Count: 2, Source: "OPD register"},
			{Date: "2026-11-04", Count: 1, Source: "OPD register"},
		}
		got := AssembleReturn(Clinic{}, records, shortTallies, m, testSettings(), asOf)

		if got.Fileable {
			t.Fatal("Fileable = true on a month with blanks, a gap, a shortfall and no clinic details")
		}
		all := strings.Join(got.Blockers, "\n")
		for _, want := range []string{
			"mandatory Form F fields blank",
			"with no Form F in the register",
			"numbering is not intact",
			"no day tally",
			"name and its registration number",
			"Appropriate Authority",
		} {
			if !strings.Contains(all, want) {
				t.Errorf("no blocker mentions %q; blockers were:\n%s", want, all)
			}
		}
	})

	t.Run("a month with nothing recorded at all", func(t *testing.T) {
		got := AssembleReturn(testClinic(), nil, nil, m, testSettings(), asOf)
		if got.Fileable {
			t.Error("Fileable = true for a month with nothing recorded at all")
		}
		if got.Totals.Entries != 0 {
			t.Errorf("Totals.Entries = %d on an empty month", got.Totals.Entries)
		}
		if !strings.Contains(strings.Join(got.Blockers, "\n"), "nil month") {
			t.Errorf("the blocker does not tell the centre how to report a nil month: %v", got.Blockers)
		}
	})

	// A centre that genuinely did nothing in a month can still report it,
	// as long as it has recorded the month as nil rather than left it
	// blank. That is the difference between a nil return and no return.
	t.Run("a month recorded as nil throughout", func(t *testing.T) {
		var nilDays []ProcedureTally
		for d := m.First(); !d.After(m.Last()); d = d.AddDate(0, 0, 1) {
			nilDays = append(nilDays, ProcedureTally{Date: FormatDay(d), Count: 0, Source: "OPD register — no procedures"})
		}
		got := AssembleReturn(testClinic(), nil, nilDays, m, testSettings(), asOf)
		if !got.Fileable {
			t.Errorf("Fileable = false for a month properly recorded as nil; blockers: %v", got.Blockers)
		}
		if !got.Check.Clear {
			t.Errorf("a month recorded as nil throughout is not clear; findings: %v", got.Check.Findings)
		}
		if len(got.Check.Days) != m.Days() {
			t.Errorf("%d days reconciled, want all %d", len(got.Check.Days), m.Days())
		}
	})
}

func TestReturnTotalsCountWhatTheyClaimTo(t *testing.T) {
	m := month(t, "2026-11")
	asOf := Day(2026, time.December, 2)

	selfRef := recordOn(2, "2026-11-04")
	selfRef.SelfReferral = true
	selfRef.ReferrerName, selfRef.ReferrerAddress, selfRef.ReferrerQualificatn = "", "", ""

	mtp := recordOn(3, "2026-11-05")
	mtp.MTPIndication = MTPIndicated
	mtp.MTPDetail = "Anomaly detected on the anomaly scan; referred for an opinion."

	thumb := recordOn(4, "2026-11-06")
	thumb.ThumbImpression = true
	thumb.IdentifierName = "Savitri Bai"
	thumb.IdentifierAge = 52
	thumb.IdentifierAddress = "12 Gokhale Path, Dharwad"
	thumb.AttestedBy = "Dr M. Rao"
	thumb.AttestedOn = "2026-11-06"

	other := recordOn(5, "2026-11-07")
	other.Procedures = []string{ProcUltrasound, ProcOther}
	other.ProcedureOther = "Transvaginal scan"

	records := []Record{recordOn(1, "2026-11-03"), selfRef, mtp, thumb, other}
	got := AssembleReturn(testClinic(), records, nil, m, testSettings(), asOf).Totals

	if got.Entries != 5 {
		t.Errorf("Entries = %d, want 5", got.Entries)
	}
	if got.SelfReferrals != 1 {
		t.Errorf("SelfReferrals = %d, want 1", got.SelfReferrals)
	}
	if got.MTPIndications != 1 {
		t.Errorf("MTPIndications = %d, want 1", got.MTPIndications)
	}
	if got.ThumbImpression != 1 {
		t.Errorf("ThumbImpression = %d, want 1", got.ThumbImpression)
	}
	if got.Ultrasound != 5 {
		t.Errorf("Ultrasound = %d, want 5", got.Ultrasound)
	}
	if got.OtherProcedures != 1 {
		t.Errorf("OtherProcedures = %d, want 1", got.OtherProcedures)
	}

	// Referrals: four by the same doctor, one self-referral, biggest first.
	if len(got.Referrals) != 2 || got.Referrals[0].Name != "Dr A. Kulkarni" || got.Referrals[0].Value != 4 {
		t.Errorf("Referrals = %+v, want Dr A. Kulkarni with 4 first", got.Referrals)
	}
	// Indications carry the number the form gives them.
	if len(got.Indications) != 1 || !strings.HasPrefix(got.Indications[0].Name, "(xvii) ") || got.Indications[0].Value != 5 {
		t.Errorf("Indications = %+v, want one line numbered (xvii) with 5", got.Indications)
	}
}
