package formf

import (
	"strings"
	"testing"
	"time"
)

func TestRetainUntil(t *testing.T) {
	cases := []struct {
		name      string
		procedure string
		years     int
		want      string
		wantOK    bool
	}{
		{"an ordinary date", "2026-11-18", 2, "2028-11-18", true},
		{"the end of a month", "2026-01-31", 2, "2028-01-31", true},

		// The case the whole clamping rule exists for. A procedure on
		// 29 February has no second anniversary, and a retention
		// deadline must not slide into March.
		{"a leap day, two years on", "2024-02-29", 2, "2026-02-28", true},
		{"a leap day, four years on", "2024-02-29", 4, "2028-02-29", true},
		{"a leap day, one year on", "2024-02-29", 1, "2025-02-28", true},
		{"28 February, two years on", "2024-02-28", 2, "2026-02-28", true},
		{"1 March after a leap day", "2024-03-01", 2, "2026-03-01", true},
		{"a leap day fifty years on, landing in a common year", "2000-02-29", 50, "2050-02-28", true},

		{"a window a district has lengthened", "2026-11-18", 5, "2031-11-18", true},
		{"a window of one year", "2026-11-18", 1, "2027-11-18", true},

		{"no procedure date at all", "", 2, "", false},
		{"a date that is not a date", "18/11/2026", 2, "", false},
		{"a date that does not exist", "2026-02-30", 2, "", false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			r := Record{ProcedureDate: c.procedure}
			s := DefaultSettings()
			s.RetentionYears = c.years

			got, ok := RetainUntil(r, s)
			if ok != c.wantOK {
				t.Fatalf("RetainUntil ok = %v, want %v", ok, c.wantOK)
			}
			if !ok {
				return
			}
			if FormatDay(got) != c.want {
				t.Errorf("RetainUntil(%s, %d years) = %s, want %s", c.procedure, c.years, FormatDay(got), c.want)
			}
		})
	}
}

// TestRetentionIsMeasuredFromTheProcedure pins the start date. Rule 9(6)
// measures from the date the procedure was completed, not from the date the
// record was written up or signed, so a late signature must not push the
// release date out.
func TestRetentionIsMeasuredFromTheProcedure(t *testing.T) {
	r := completeRecord()
	r.ProcedureDate = "2026-11-18"
	r.SignedOn = "2026-11-23"

	completed, ok := CompletionDate(r)
	if !ok {
		t.Fatal("CompletionDate gave up on a record with a procedure date")
	}
	if FormatDay(completed) != "2026-11-18" {
		t.Errorf("CompletionDate = %s, want the procedure date 2026-11-18", FormatDay(completed))
	}

	st := RetentionOn(r, DefaultSettings(), Day(2026, time.November, 24))
	if st.RetainUntil != "2028-11-18" {
		t.Errorf("RetainUntil = %s, want 2028-11-18", st.RetainUntil)
	}
	if !strings.Contains(st.Basis, "item 13") {
		t.Errorf("Basis = %q, want it to name the item the date comes from", st.Basis)
	}
}

func TestRetentionOn(t *testing.T) {
	cases := []struct {
		name            string
		procedure       string
		proceedings     bool
		asOf            string
		wantUntil       string
		wantDays        int
		wantReleasable  bool
		wantOpenEnded   bool
		wantNoteContain string
	}{
		{
			name: "well inside the window", procedure: "2026-11-18", asOf: "2026-11-24",
			wantUntil: "2028-11-18", wantDays: 725, wantNoteContain: "Hold until 2028-11-18",
		},
		{
			name: "on the last day of the window", procedure: "2024-11-24", asOf: "2026-11-24",
			wantUntil: "2026-11-24", wantDays: 0, wantNoteContain: "Hold until 2026-11-24",
		},
		{
			name: "the day after the window closed", procedure: "2024-11-23", asOf: "2026-11-24",
			wantUntil: "2026-11-23", wantDays: -1, wantReleasable: true,
			wantNoteContain: "window closed on 2026-11-23",
		},
		{
			name: "long past the window", procedure: "2019-01-01", asOf: "2026-11-24",
			wantUntil: "2021-01-01", wantDays: -2153, wantReleasable: true,
			wantNoteContain: "confirm with the Appropriate Authority",
		},
		{
			// The proviso. Proceedings hold the record for the longer
			// of the two periods, so the ordinary date is still
			// computed and shown — it just does not release anything.
			name: "proceedings noted, window still open", procedure: "2026-11-18", proceedings: true, asOf: "2026-11-24",
			wantUntil: "2028-11-18", wantDays: 725, wantOpenEnded: true,
			wantNoteContain: "whichever is later",
		},
		{
			name: "proceedings noted, window long closed", procedure: "2019-01-01", proceedings: true, asOf: "2026-11-24",
			wantUntil: "2021-01-01", wantDays: -2153, wantOpenEnded: true,
			wantNoteContain: "finally disposed of",
		},
		{
			name: "a leap-day procedure on the day its window closes", procedure: "2024-02-29", asOf: "2026-02-28",
			wantUntil: "2026-02-28", wantDays: 0, wantNoteContain: "Hold until 2026-02-28",
		},
		{
			name: "a leap-day procedure the day after", procedure: "2024-02-29", asOf: "2026-03-01",
			wantUntil: "2026-02-28", wantDays: -1, wantReleasable: true,
		},
		{
			name: "no procedure date", procedure: "", asOf: "2026-11-24",
			wantNoteContain: "no procedure date",
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			asOf, err := ParseDay(c.asOf)
			if err != nil {
				t.Fatalf("fixture %q: %v", c.asOf, err)
			}
			r := Record{Serial: 7, ProcedureDate: c.procedure, ProceedingsPending: c.proceedings}
			got := RetentionOn(r, DefaultSettings(), asOf)

			if got.Serial != 7 {
				t.Errorf("Serial = %d, want 7", got.Serial)
			}
			if got.RetainUntil != c.wantUntil {
				t.Errorf("RetainUntil = %q, want %q", got.RetainUntil, c.wantUntil)
			}
			if c.wantUntil != "" && got.DaysRemaining != c.wantDays {
				t.Errorf("DaysRemaining = %d, want %d", got.DaysRemaining, c.wantDays)
			}
			if got.Releasable != c.wantReleasable {
				t.Errorf("Releasable = %v, want %v", got.Releasable, c.wantReleasable)
			}
			if got.OpenEnded != c.wantOpenEnded {
				t.Errorf("OpenEnded = %v, want %v", got.OpenEnded, c.wantOpenEnded)
			}
			if got.OpenEnded && got.Releasable {
				t.Error("an entry with proceedings noted against it reported itself releasable")
			}
			if got.Years != RetentionYears {
				t.Errorf("Years = %d, want the default %d", got.Years, RetentionYears)
			}
			if c.wantNoteContain != "" && !strings.Contains(got.Note, c.wantNoteContain) {
				t.Errorf("Note = %q, want it to contain %q", got.Note, c.wantNoteContain)
			}
		})
	}
}

func TestSummariseRetention(t *testing.T) {
	asOf := Day(2026, time.November, 24)
	records := []Record{
		{Serial: 1, ProcedureDate: "2026-11-18"},                           // holding
		{Serial: 2, ProcedureDate: "2026-06-01"},                           // holding, earlier deadline
		{Serial: 3, ProcedureDate: "2020-01-01"},                           // closed
		{Serial: 4, ProcedureDate: "2021-05-05"},                           // closed
		{Serial: 5, ProcedureDate: "2019-07-07", ProceedingsPending: true}, // open-ended
		{Serial: 6, ProcedureDate: ""},                                     // no date
		{Serial: 7, ProcedureDate: "2026-11-01", ProceedingsPending: true}, // open-ended
	}
	got := SummariseRetention(records, DefaultSettings(), asOf)

	if got.Total != 7 {
		t.Errorf("Total = %d, want 7", got.Total)
	}
	if got.Holding != 2 {
		t.Errorf("Holding = %d, want 2", got.Holding)
	}
	if got.Releasable != 2 {
		t.Errorf("Releasable = %d, want 2", got.Releasable)
	}
	if got.OpenEnded != 2 {
		t.Errorf("OpenEnded = %d, want 2", got.OpenEnded)
	}
	if got.Undatable != 1 {
		t.Errorf("Undatable = %d, want 1", got.Undatable)
	}
	if got.Holding+got.Releasable+got.OpenEnded+got.Undatable != got.Total {
		t.Error("the four counts do not add up to the total")
	}
	if got.EarliestDue != "2028-06-01" {
		t.Errorf("EarliestDue = %q, want 2028-06-01 (the earliest deadline among the entries still being held)", got.EarliestDue)
	}
	if got.AsOf != "2026-11-24" {
		t.Errorf("AsOf = %q, want 2026-11-24", got.AsOf)
	}
	if !strings.Contains(got.Reference, "rule 9(6)") {
		t.Errorf("Reference = %q, want it to cite the rule", got.Reference)
	}
}

func TestSummariseRetentionOfAnEmptyRegister(t *testing.T) {
	got := SummariseRetention(nil, DefaultSettings(), Day(2026, time.November, 24))
	if got.Total != 0 || got.Holding != 0 || got.EarliestDue != "" {
		t.Errorf("an empty register summarised as %+v", got)
	}
}

func TestSettingsNormalised(t *testing.T) {
	cases := []struct {
		name                            string
		in                              Settings
		wantYears, wantDay, wantOpening int
	}{
		{"the defaults pass through", DefaultSettings(), 2, 5, 1},
		{"a zero value becomes the defaults", Settings{}, 2, 5, 1},
		{"a negative retention window", Settings{RetentionYears: -3}, 2, 5, 1},
		{"an absurd retention window", Settings{RetentionYears: 400}, 2, 5, 1},
		{"a district that requires five years", Settings{RetentionYears: 5}, 5, 5, 1},
		{"a due day of zero", Settings{ReturnDueDay: 0}, 2, 5, 1},
		{"a due day past the end of February", Settings{ReturnDueDay: 31}, 2, 5, 1},
		{"a due day of ten", Settings{ReturnDueDay: 10}, 2, 10, 1},
		{"an opening serial carried over from paper", Settings{OpeningSerial: 412}, 2, 5, 412},
		{"a negative opening serial", Settings{OpeningSerial: -5}, 2, 5, 1},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := c.in.Normalised()
			if got.RetentionYears != c.wantYears {
				t.Errorf("RetentionYears = %d, want %d", got.RetentionYears, c.wantYears)
			}
			if got.ReturnDueDay != c.wantDay {
				t.Errorf("ReturnDueDay = %d, want %d", got.ReturnDueDay, c.wantDay)
			}
			if got.OpeningSerial != c.wantOpening {
				t.Errorf("OpeningSerial = %d, want %d", got.OpeningSerial, c.wantOpening)
			}
		})
	}
}

func TestSettingsOverridden(t *testing.T) {
	cases := []struct {
		name string
		in   Settings
		want bool
	}{
		{"the defaults", DefaultSettings(), false},
		{"a changed retention window", Settings{RetentionYears: 5, ReturnDueDay: 5, OpeningSerial: 1}, true},
		{"a changed due day", Settings{RetentionYears: 2, ReturnDueDay: 7, OpeningSerial: 1}, true},
		{"only the opening serial changed, which is not a statutory figure",
			Settings{RetentionYears: 2, ReturnDueDay: 5, OpeningSerial: 412}, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := c.in.Overridden(); got != c.want {
				t.Errorf("Overridden() = %v, want %v", got, c.want)
			}
		})
	}
}

// TestInspectionSample covers the sampling rule the Ministry's guidance sets:
// thirty forms, or a tenth of them, whichever is more.
func TestInspectionSample(t *testing.T) {
	cases := []struct {
		entries, want int
	}{
		{0, 0},
		{1, 1},
		{12, 12},
		{30, 30},
		{31, 30},
		{299, 30},
		{300, 30},
		{301, 31},
		{450, 45},
		{1000, 100},
		{1001, 101},
		{-5, 0},
	}
	for _, c := range cases {
		got := InspectionSample(c.entries)
		if got != c.want {
			t.Errorf("InspectionSample(%d) = %d, want %d", c.entries, got, c.want)
		}
		if got > c.entries && c.entries > 0 {
			t.Errorf("InspectionSample(%d) = %d, which is more entries than the register holds", c.entries, got)
		}
	}
}

// TestStatutoryFiguresAreWhatTheResearchSays is a tripwire. If somebody edits
// a figure, this test fails and the documentation has to be revisited with it.
func TestStatutoryFiguresAreWhatTheResearchSays(t *testing.T) {
	if RetentionYears != 2 {
		t.Errorf("RetentionYears = %d; rule 9(6) and section 29(1) both put it at 2", RetentionYears)
	}
	if DefaultReturnDueDay != 5 {
		t.Errorf("DefaultReturnDueDay = %d; rule 9(8) puts it at the 5th of the following month", DefaultReturnDueDay)
	}
	if InspectionSampleFloor != 30 {
		t.Errorf("InspectionSampleFloor = %d, want 30", InspectionSampleFloor)
	}
	if InspectionSampleShare != 0.10 {
		t.Errorf("InspectionSampleShare = %v, want 0.10", InspectionSampleShare)
	}
	for name, ref := range map[string]string{
		"RetentionReference":        RetentionReference,
		"RetentionProviso":          RetentionProviso,
		"ReturnDueReference":        ReturnDueReference,
		"ReturnFormatNote":          ReturnFormatNote,
		"PrintedCopyReference":      PrintedCopyReference,
		"InspectionSampleReference": InspectionSampleReference,
		"Consequences":              Consequences,
	} {
		if len(ref) < 60 {
			t.Errorf("%s is too short to carry a citation: %q", name, ref)
		}
	}
	// Each figure has to travel with its source.
	if !strings.Contains(RetentionReference, "rule 9(6)") || !strings.Contains(RetentionReference, "s.29(1)") {
		t.Errorf("RetentionReference does not cite both provisions: %q", RetentionReference)
	}
	if !strings.Contains(ReturnDueReference, "rule 9(8)") {
		t.Errorf("ReturnDueReference does not cite the rule: %q", ReturnDueReference)
	}
	if !strings.Contains(PrintedCopyReference, "rule 9(7)") {
		t.Errorf("PrintedCopyReference does not cite the rule: %q", PrintedCopyReference)
	}
	// The format of the monthly report is the one thing this tool must not
	// claim to know.
	if !strings.Contains(ReturnFormatNote, "prescribes no format") {
		t.Errorf("ReturnFormatNote no longer admits that no format is prescribed: %q", ReturnFormatNote)
	}
}
