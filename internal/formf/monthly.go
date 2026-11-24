package formf

import (
	"fmt"
	"sort"
	"time"
)

// The monthly cycle.
//
// Rule 9(8) requires every ultrasound clinic and imaging centre to send the
// Appropriate Authority a complete report of all the procedures it conducted
// in a month, by the fifth day of the following month. Two different things
// have to be true before such a report can be sent, and it is worth keeping
// them apart.
//
// The first is that the register is complete: that every procedure the centre
// actually carried out has a Form F against it. A register cannot answer that
// out of its own entries, because a Form F that was never written is invisible
// to the register that does not hold it. The Ministry's own inspection
// guidance says how the question is settled — tally Form F against the OPD
// register, so that the number of patients examined and the number of
// statutory forms filled can be seen to agree. So the centre enters its own
// day count here, from the OPD register, the appointment book or the machine's
// log, and the month's reconciliation compares the two numbers day by day.
// Three scans logged on the 14th against two Form F entries dated the 14th is
// the finding an inspection would make, found a fortnight earlier and by the
// centre itself.
//
// The second is the report. It is assembled from exactly the entries whose
// procedure date falls inside the month — no adjacent-month strays, no
// carry-forward, nothing added by hand to make the numbers agree. Where
// anything is missing the report is marked as not ready to send and the
// reasons are listed, because a report that quietly leaves out the gaps is
// worse than no report at all.

// ProcedureTally is the centre's own count of procedures carried out on one
// day, taken from a source outside this register — the OPD register that rule
// 9(1) requires, the appointment book, or the machine's own log. It is the
// independent number the register is measured against, and it is the only
// reason the monthly check can find a Form F that was never written at all.
type ProcedureTally struct {
	Date   string `json:"date"`
	Count  int    `json:"count"`
	Source string `json:"source"`
	Note   string `json:"note,omitempty"`
}

// DayStatus is the outcome of reconciling one day.
type DayStatus string

const (
	// DayMatched means the tally and the register agree.
	DayMatched DayStatus = "matched"
	// DayFormsMissing means procedures were logged that have no Form F.
	// This is the finding that seals machines.
	DayFormsMissing DayStatus = "forms-missing"
	// DayUntallied means the register holds entries for a day the clinic
	// never tallied, so there is nothing to check them against.
	DayUntallied DayStatus = "untallied"
	// DayOverTallied means more Form F entries than logged procedures,
	// which points at the tally rather than the register.
	DayOverTallied DayStatus = "over-tallied"
)

// DayReconciliation is one day of the month, both numbers side by side.
type DayReconciliation struct {
	Date    string    `json:"date"`
	Weekday string    `json:"weekday"`
	Tallied int       `json:"tallied"`
	Entries int       `json:"entries"`
	Serials []int     `json:"serials,omitempty"`
	Status  DayStatus `json:"status"`
	Source  string    `json:"source,omitempty"`
	Message string    `json:"message,omitempty"`
}

// MonthCheck is the completeness position of one month.
type MonthCheck struct {
	Month        string              `json:"month"`
	MonthLabel   string              `json:"monthLabel"`
	Tallied      int                 `json:"talliedProcedures"`
	Entries      int                 `json:"formFEntries"`
	FormsMissing int                 `json:"formsMissing"`
	Untallied    int                 `json:"untalliedEntries"`
	Incomplete   []int               `json:"incompleteSerials"`
	Days         []DayReconciliation `json:"days"`
	Serial       SerialCheck         `json:"serial"`
	TallyDays    int                 `json:"tallyDays"`
	Clear        bool                `json:"clear"`
	Findings     []string            `json:"findings"`
}

// CheckMonth reconciles a month: the clinic's own day tally against the Form F
// entries dated in that month, plus the completeness of each of those entries
// and the integrity of the numbering across the whole register.
//
// records is the whole register, not a pre-filtered month, so that serial
// integrity is judged against the real sequence and so that the filter itself
// is this function's business and cannot be got wrong by a caller.
func CheckMonth(records []Record, tallies []ProcedureTally, m Month, s Settings) MonthCheck {
	s = s.Normalised()
	out := MonthCheck{Month: m.String(), MonthLabel: m.Label(), Clear: true}
	if !m.Valid() {
		out.Clear = false
		out.Findings = []string{"No month was given to check."}
		return out
	}

	inMonth := EntriesForMonth(records, m)
	out.Entries = len(inMonth)
	out.Serial = CheckSerials(records, s)

	// Index the register's entries by day.
	byDay := map[string][]int{}
	for _, r := range inMonth {
		d, err := ParseDay(r.ProcedureDate)
		if err != nil {
			continue
		}
		k := FormatDay(d)
		byDay[k] = append(byDay[k], r.Serial)
	}
	for k := range byDay {
		sort.Ints(byDay[k])
	}

	// Index the tally by day, summing repeated entries for the same date so
	// that a clinic entering a morning and an evening figure is not punished
	// for it.
	talliedByDay := map[string]int{}
	sourceByDay := map[string]string{}
	for _, t := range tallies {
		d, err := ParseDay(t.Date)
		if err != nil || !m.Contains(d) || t.Count < 0 {
			continue
		}
		k := FormatDay(d)
		talliedByDay[k] += t.Count
		if t.Source != "" {
			sourceByDay[k] = t.Source
		}
	}
	out.TallyDays = len(talliedByDay)

	for day := m.First(); !day.After(m.Last()); day = day.AddDate(0, 0, 1) {
		k := FormatDay(day)
		tallied, hasTally := talliedByDay[k]
		entries := len(byDay[k])
		if !hasTally && entries == 0 {
			continue // an ordinary quiet day: nothing logged, nothing entered
		}
		rec := DayReconciliation{
			Date:    k,
			Weekday: day.Weekday().String(),
			Tallied: tallied,
			Entries: entries,
			Serials: byDay[k],
			Source:  sourceByDay[k],
		}
		switch {
		case !hasTally:
			rec.Status = DayUntallied
			rec.Message = fmt.Sprintf("%d Form F entries are dated %s but no day tally was recorded for that date, so there is nothing to check them against. Enter the machine-log count for the day.", entries, k)
			out.Untallied += entries
			out.Clear = false
		case tallied > entries:
			rec.Status = DayFormsMissing
			rec.Message = fmt.Sprintf("%d procedures logged on %s, %d Form F entries. %d Form F are missing for that day.", tallied, k, entries, tallied-entries)
			out.FormsMissing += tallied - entries
			out.Clear = false
		case entries > tallied:
			rec.Status = DayOverTallied
			rec.Message = fmt.Sprintf("%d Form F entries are dated %s but only %d procedures were logged. Check the day tally: a register cannot hold more procedures than were carried out.", entries, k, tallied)
			out.Clear = false
		default:
			rec.Status = DayMatched
		}
		out.Days = append(out.Days, rec)
		out.Tallied += tallied
	}

	for _, r := range inMonth {
		if !Validate(r).Complete() {
			out.Incomplete = append(out.Incomplete, r.Serial)
		}
	}
	sort.Ints(out.Incomplete)

	// The findings, loudest first.
	if out.FormsMissing > 0 {
		out.Findings = append(out.Findings, fmt.Sprintf("%d procedure(s) carried out in %s have no Form F in this register. A procedure without a Form F is the single most common finding at inspection, and the consequences reach as far as sealing the machine and suspension of registration. Write the missing Form F before the month's report goes in.", out.FormsMissing, m.Label()))
	}
	if n := len(out.Incomplete); n > 0 {
		out.Findings = append(out.Findings, fmt.Sprintf("%d Form F entr%s in %s %s mandatory fields left blank: serial %s. An incomplete Form F is treated as a Form F not maintained.", n, plural(n, "y", "ies"), m.Label(), plural(n, "has", "have"), joinInts(out.Incomplete)))
		out.Clear = false
	}
	if out.Untallied > 0 {
		out.Findings = append(out.Findings, fmt.Sprintf("%d entr%s fall on days with no day tally, so completeness for those days is unverified.", out.Untallied, plural(out.Untallied, "y", "ies")))
	}
	for _, iss := range out.Serial.NumberingIssues() {
		out.Findings = append(out.Findings, "Numbering: "+iss.Message)
		out.Clear = false
	}
	// An entry whose date runs backwards against its number is reported but
	// does not stop the month: see the note on SerialCheck.Ordered.
	for _, iss := range out.Serial.OrderIssues() {
		out.Findings = append(out.Findings, "Worth explaining: "+iss.Message)
	}
	// Nothing recorded at all is different from a month recorded as nil. A
	// centre that counted its quiet days has shown the month was quiet; a
	// centre that recorded nothing has shown nothing either way, and from
	// outside the two look identical.
	if out.Entries == 0 && out.TallyDays == 0 {
		out.Findings = append(out.Findings, fmt.Sprintf("Nothing at all is recorded for %s — no Form F entries and no day counts. If the centre carried out no procedures that month, enter the days as nil so that the month says so. A silent month and a nil month look the same from the outside, and only one of them is a record.", m.Label()))
		out.Clear = false
	}
	return out
}

// EntriesForMonth returns exactly the entries whose procedure date falls in
// the month, in serial order. An entry with an unparseable or empty procedure
// date belongs to no month and is deliberately not swept into one: it shows up
// instead as an incomplete entry, which is what it is.
func EntriesForMonth(records []Record, m Month) []Record {
	if !m.Valid() {
		return nil
	}
	var out []Record
	for _, r := range records {
		d, err := ParseDay(r.ProcedureDate)
		if err != nil || !m.Contains(d) {
			continue
		}
		out = append(out, r)
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Serial < out[j].Serial })
	return out
}

// Count is one line of a tally, kept as a slice rather than a map so that
// every printed return comes out in the same order.
type Count struct {
	Name  string `json:"name"`
	Value int    `json:"value"`
}

// ReturnTotals is the arithmetic on the face of the report.
type ReturnTotals struct {
	Entries         int     `json:"entries"`
	Procedures      []Count `json:"procedures"`
	Indications     []Count `json:"indications"`
	Referrals       []Count `json:"referrals"`
	Ultrasound      int     `json:"ultrasound"`
	OtherProcedures int     `json:"otherProcedures"`
	SelfReferrals   int     `json:"selfReferrals"`
	MTPIndications  int     `json:"mtpIndications"`
	ThumbImpression int     `json:"thumbImpression"`
	FirstDate       string  `json:"firstDate,omitempty"`
	LastDate        string  `json:"lastDate,omitempty"`
	SerialFirst     int     `json:"serialFirst,omitempty"`
	SerialLast      int     `json:"serialLast,omitempty"`
	DistinctDays    int     `json:"distinctDays"`
}

// MonthlyReturn is the month's report, assembled.
type MonthlyReturn struct {
	Clinic     Clinic       `json:"clinic"`
	Month      string       `json:"month"`
	MonthLabel string       `json:"monthLabel"`
	DueOn      string       `json:"dueOn"`
	DueDay     int          `json:"dueDay"`
	DaysToDue  int          `json:"daysToDue"`
	Overdue    bool         `json:"overdue"`
	PreparedOn string       `json:"preparedOn"`
	Entries    []Record     `json:"entries"`
	Totals     ReturnTotals `json:"totals"`
	Check      MonthCheck   `json:"check"`
	Fileable   bool         `json:"fileable"`
	Blockers   []string     `json:"blockers"`
	Reference  string       `json:"reference"`
	FormatNote string       `json:"formatNote"`
	Settings   Settings     `json:"settings"`
}

// ReturnDueDate returns the date the month's report is due with the
// Appropriate Authority: the configured day of the following month.
func ReturnDueDate(m Month, s Settings) time.Time {
	s = s.Normalised()
	n := m.Next()
	day := s.ReturnDueDay
	if max := n.Days(); day > max {
		day = max
	}
	return Day(n.Year, n.Mon, day)
}

// AssembleReturn builds the month's return from exactly the entries dated in
// that month, and refuses to call it fileable while anything is missing.
func AssembleReturn(c Clinic, records []Record, tallies []ProcedureTally, m Month, s Settings, asOf time.Time) MonthlyReturn {
	s = s.Normalised()
	entries := EntriesForMonth(records, m)
	due := ReturnDueDate(m, s)
	out := MonthlyReturn{
		Clinic:     c,
		Month:      m.String(),
		MonthLabel: m.Label(),
		DueOn:      FormatDay(due),
		DueDay:     s.ReturnDueDay,
		DaysToDue:  DaysBetween(dayOf(asOf), due),
		PreparedOn: FormatDay(asOf),
		Entries:    entries,
		Totals:     totalsFor(entries),
		Check:      CheckMonth(records, tallies, m, s),
		Reference:  ReturnDueReference,
		FormatNote: ReturnFormatNote,
		Settings:   s,
	}
	out.Overdue = out.DaysToDue < 0

	// Blockers. Each one is a reason this return would be wrong if sent.
	if n := len(out.Check.Incomplete); n > 0 {
		out.Blockers = append(out.Blockers, fmt.Sprintf("%d entr%s in the month %s mandatory Form F fields blank (serial %s). Complete %s before sending the report.", n, plural(n, "y", "ies"), plural(n, "has", "have"), joinInts(out.Check.Incomplete), plural(n, "it", "them")))
	}
	if out.Check.FormsMissing > 0 {
		out.Blockers = append(out.Blockers, fmt.Sprintf("The day tally accounts for %d procedure(s) in the month with no Form F in the register. The report would under-state the month.", out.Check.FormsMissing))
	}
	if !out.Check.Serial.Intact {
		out.Blockers = append(out.Blockers, fmt.Sprintf("The register's numbering is not intact: %d problem(s) to resolve first. A gap reads as a removed record, and a report sent over one invites the question.", len(out.Check.Serial.NumberingIssues())))
	}
	if out.Check.Untallied > 0 {
		out.Blockers = append(out.Blockers, fmt.Sprintf("%d entr%s fall on days with no day tally. Enter the machine-log count for those days so the month can be shown complete.", out.Check.Untallied, plural(out.Check.Untallied, "y", "ies")))
	}
	if c.Name == "" || c.RegistrationNo == "" {
		out.Blockers = append(out.Blockers, "The centre's name and its registration number under the Act have to be on the face of the report. Fill them in under settings.")
	}
	if c.AppropriateAuthority == "" {
		out.Blockers = append(out.Blockers, "The report is addressed to the Appropriate Authority for the district. Name that office under settings.")
	}
	if out.Check.Entries == 0 && out.Check.TallyDays == 0 {
		out.Blockers = append(out.Blockers, fmt.Sprintf("Nothing at all is recorded for %s. A report has to say something: if the centre carried out no procedures, enter the month's days as nil so that this report states a nil month rather than leaving it blank.", m.Label()))
	}
	out.Fileable = len(out.Blockers) == 0
	return out
}

func totalsFor(entries []Record) ReturnTotals {
	out := ReturnTotals{Entries: len(entries)}
	procedures := map[string]int{}
	indications := map[string]int{}
	referrals := map[string]int{}
	days := map[string]bool{}
	for _, r := range entries {
		for _, p := range r.Procedures {
			procedures[p]++
			switch p {
			case ProcUltrasound:
				out.Ultrasound++
			default:
				out.OtherProcedures++
			}
		}
		for _, i := range r.Indications {
			if n := IndicationNumber(i); n != "" && n != "—" {
				indications[n+" "+i]++
				continue
			}
			indications[i]++
		}
		if r.SelfReferral {
			referrals["Self-referral"]++
			out.SelfReferrals++
		} else if r.ReferrerName != "" {
			referrals[r.ReferrerName]++
		} else {
			referrals["Not recorded"]++
		}
		if r.MTPIndication == MTPIndicated {
			out.MTPIndications++
		}
		if r.ThumbImpression {
			out.ThumbImpression++
		}
		if d, err := ParseDay(r.ProcedureDate); err == nil {
			k := FormatDay(d)
			days[k] = true
			if out.FirstDate == "" || k < out.FirstDate {
				out.FirstDate = k
			}
			if k > out.LastDate {
				out.LastDate = k
			}
		}
		if out.SerialFirst == 0 || r.Serial < out.SerialFirst {
			out.SerialFirst = r.Serial
		}
		if r.Serial > out.SerialLast {
			out.SerialLast = r.Serial
		}
	}
	out.DistinctDays = len(days)
	out.Procedures = countsOf(procedures)
	out.Indications = countsOf(indications)
	out.Referrals = countsOf(referrals)
	return out
}

// countsOf turns a tally map into a stable slice: biggest first, ties broken
// alphabetically, so that two runs of the same month produce the same page.
func countsOf(m map[string]int) []Count {
	out := make([]Count, 0, len(m))
	for k, v := range m {
		out = append(out, Count{Name: k, Value: v})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Value != out[j].Value {
			return out[i].Value > out[j].Value
		}
		return out[i].Name < out[j].Name
	})
	return out
}

func plural(n int, one, many string) string {
	if n == 1 {
		return one
	}
	return many
}

func joinInts(ns []int) string {
	s := ""
	for i, n := range ns {
		if i > 0 {
			s += ", "
		}
		s += fmt.Sprint(n)
	}
	return s
}
