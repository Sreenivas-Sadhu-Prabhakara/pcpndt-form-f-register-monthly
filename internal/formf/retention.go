package formf

import (
	"fmt"
	"math"
	"time"
)

// The statutory figures, in one place.
//
// Four numbers govern this register: how long a record is kept, when the
// month's return is due, how many entries an inspection looks at, and the age
// at which age is itself an indication. Each one is a named constant with the
// provision it comes from beside it, each is stated in the documentation, and
// the first three are overridable in the register's settings — because a
// figure set by rule can be changed by rule, and a tool that buried its
// numbers in a dozen call sites would go stale without anyone noticing.
//
// Every figure here is INDICATIVE. Confirm what applies in your district with
// your Appropriate Authority. Nothing in this tool treats its own defaults as
// authoritative, and nothing here is legal advice.

// RetentionYears is the retention window this register applies to a Form F
// record and the papers filed with it.
//
// Rule 9(6) of the Rules puts it at two years from the date the procedure was
// completed, and section 29(1) of the Act requires the same two years for all
// records, charts, forms, reports and consent letters, leaving room for a
// longer period to be prescribed. No longer period has been prescribed.
const RetentionYears = 2

// RetentionReference is the provision the window comes from. The figure and
// its source travel together so that neither can be quoted without the other.
const RetentionReference = "PNDT Rules 1996 (as amended), rule 9(6) — case related records, forms of consent, laboratory results, sonographic plates or slides, recommendations and letters to be preserved for two years from the date of completion of the procedure or test; and PC&PNDT Act 1994, s.29(1), to the same effect for all records, charts, forms, reports and consent letters"

// RetentionProviso is the part of the same provisions that no arithmetic can
// express: live proceedings stop the clock.
const RetentionProviso = "Rule 9(6) — in the event of any legal proceedings, records are preserved until those proceedings are finally disposed of or until the two years expire, whichever is later; and PC&PNDT Act 1994, s.29(1) proviso, to the same effect where proceedings are instituted against the centre"

// DefaultReturnDueDay is the day of the following month by which the month's
// report has to reach the Appropriate Authority.
const DefaultReturnDueDay = 5

// ReturnDueReference is the provision the monthly cycle comes from. Rule 9(8)
// is not in the Rules as originally made: it was inserted in 2003.
const ReturnDueReference = "PNDT Rules 1996, rule 9(8) (inserted 2003) — every Ultrasound Clinic and Imaging Centre shall send a complete report in respect of all pre-conception or pregnancy related procedures, techniques and tests conducted by them in respect of each month, by the 5th day of the following month, to the concerned Appropriate Authority"

// ReturnFormatNote is a thing this tool will not pretend to know. The rule
// requires "a complete report" and prescribes no format for it.
const ReturnFormatNote = "Rule 9(8) prescribes no format for the monthly report. It requires a complete report of all procedures conducted in the month. Districts and states direct variously that it be filed as copies of the month's Form F, as a consolidated statement, or both, and several states run their own online portal with its own cut-off. Ask your Appropriate Authority which format that office requires, and copy this report onto it."

// PrintedCopyReference is why a register kept on a computer is not the end of
// the matter. It is the reason this tool prints.
const PrintedCopyReference = "PNDT Rules 1996, rule 9(7) — where records are maintained on computer or other electronic equipment, a printed copy of the record shall be taken and preserved after authentication by a person responsible for such record"

// InspectionSampleFloor and InspectionSampleShare are the sampling rule an
// inspection works to.
//
// The Ministry's Standard Operating Guidelines for District Appropriate
// Authorities direct that 30 Form F, or 10 per cent of them, whichever is
// more, be scrutinised at an inspection. Knowing the number is useful: it is
// how many entries have to stand up, not how many exist.
const (
	InspectionSampleFloor = 30
	InspectionSampleShare = 0.10
)

// InspectionSampleReference names the source of the sampling rule.
const InspectionSampleReference = "Ministry of Health and Family Welfare, Standard Operating Guidelines for District Appropriate Authorities (2016) — 30 Form F or 10% of them, whichever is more, to be scrutinised during an inspection"

// AdvancedMaternalAge is the age at which age is itself an indication for a
// pre-natal diagnostic procedure, as item 19B of the form puts it. It is not
// used to require anything; the register notes it so that an entry for a woman
// of that age is not left with a weaker indication than it could carry.
const AdvancedMaternalAge = 35

// Consequences states what a deficient record exposes the centre to. It is
// here as a constant because it is the reason the completeness check exists,
// and because a tool that warns about consequences should be precise about
// which ones.
const Consequences = "Under the proviso to section 4(3) of the Act, any deficiency or inaccuracy in the record kept of an ultrasound on a pregnant woman amounts to a contravention of section 5 or section 6 unless the person conducting it proves otherwise. " +
	"A contravention of the Act or the rules by a practitioner, owner or employee of a centre is punishable under section 23(1) with imprisonment of up to three years and a fine of up to ten thousand rupees, and up to five years and fifty thousand rupees on a subsequent conviction, with the practitioner's name reported to the State Medical Council. " +
	"Registration can be suspended or cancelled under section 20, and under section 20(3) suspended without a show-cause notice where the Appropriate Authority considers it necessary in the public interest. " +
	"Records, machines and equipment can be seized and sealed under section 30 read with rule 12. " +
	"These are the provisions as this tool understands them and not legal advice; take advice on your own position."

// InspectionSample returns how many Form F entries an inspection would
// scrutinise out of a register of n entries: the floor, or the share of the
// register, whichever is more, and never more than the register holds.
func InspectionSample(n int) int {
	if n <= 0 {
		return 0
	}
	share := int(math.Ceil(float64(n) * InspectionSampleShare))
	sample := InspectionSampleFloor
	if share > sample {
		sample = share
	}
	if sample > n {
		sample = n
	}
	return sample
}

// Settings holds the figures a clinic may need to change without touching the
// code.
type Settings struct {
	// RetentionYears is the retention window in whole calendar years.
	RetentionYears int `json:"retentionYears"`
	// ReturnDueDay is the day of the following month the report is due on.
	ReturnDueDay int `json:"returnDueDay"`
	// OpeningSerial is the serial number the first entry in this register
	// carries: 1 for a new register, or the number after the last paper
	// entry for a register carried over from paper.
	OpeningSerial int `json:"openingSerial"`
	// ResponsibleFor is the person who authenticates the printed copy
	// rule 9(7) requires. It is a name, printed on every page.
	ResponsiblePerson string `json:"responsiblePerson"`
}

// DefaultSettings returns the indicative defaults.
func DefaultSettings() Settings {
	return Settings{
		RetentionYears: RetentionYears,
		ReturnDueDay:   DefaultReturnDueDay,
		OpeningSerial:  1,
	}
}

// Normalised returns the settings with any unusable value replaced by the
// default, so that a hand-edited data file cannot produce a negative retention
// window or a report due on the thirty-eighth of the month.
func (s Settings) Normalised() Settings {
	d := DefaultSettings()
	if s.RetentionYears <= 0 || s.RetentionYears > 99 {
		s.RetentionYears = d.RetentionYears
	}
	if s.ReturnDueDay < 1 || s.ReturnDueDay > 28 {
		s.ReturnDueDay = d.ReturnDueDay
	}
	if s.OpeningSerial < 1 {
		s.OpeningSerial = d.OpeningSerial
	}
	return s
}

// Overridden reports whether the settings depart from the indicative
// defaults, so that a printed page can say so rather than implying that this
// tool's defaults are law.
func (s Settings) Overridden() bool {
	d := DefaultSettings()
	return s.RetentionYears != d.RetentionYears || s.ReturnDueDay != d.ReturnDueDay
}

// Retention is the retention position of one record on one day.
type Retention struct {
	Serial int `json:"serial"`
	// CompletedOn is the date the window is measured from: the date the
	// procedure was completed.
	CompletedOn string `json:"completedOn"`
	// Basis says, in one line, which date that is.
	Basis string `json:"basis"`
	// RetainUntil is the last date the record must still be held,
	// inclusive. It is set even where proceedings are noted, because the
	// rule holds records for the longer of the two periods.
	RetainUntil string `json:"retainUntil"`
	// Years is the window actually applied, so that a printed page carries
	// the figure used rather than the figure in the code.
	Years int `json:"years"`
	// DaysRemaining counts from the reference day to RetainUntil. It is
	// zero on the last day and negative after it.
	DaysRemaining int `json:"daysRemaining"`
	// OpenEnded is true where proceedings are noted against the record, in
	// which case no release date applies at all.
	OpenEnded bool `json:"openEnded"`
	// Releasable is true only where the window has closed and no
	// proceedings are noted.
	Releasable bool   `json:"releasable"`
	Note       string `json:"note"`
}

// CompletionDate returns the date the retention window runs from: the date the
// procedure was completed, which is item 13 of the form and what rule 9(6)
// measures from. An entry with no usable procedure date has no retention
// position, which is itself something to fix.
func CompletionDate(r Record) (time.Time, bool) {
	d, err := ParseDay(r.ProcedureDate)
	if err != nil {
		return time.Time{}, false
	}
	return d, true
}

// RetainUntil returns the last date a record must still be held, inclusive.
//
// The window is a whole number of calendar years added to the date the
// procedure was completed, clamped so that a record completed on a leap day
// does not have its deadline pushed into the following month. The date is
// inclusive and therefore one day conservative: a two-year window on a
// procedure completed on 29 February 2024 runs to 28 February 2026, and the
// register will not call that record releasable until 1 March 2026.
func RetainUntil(r Record, s Settings) (time.Time, bool) {
	completed, ok := CompletionDate(r)
	if !ok {
		return time.Time{}, false
	}
	return AddYearsClamped(completed, s.Normalised().RetentionYears), true
}

// RetentionOn returns the retention position of a record on a given day.
func RetentionOn(r Record, s Settings, asOf time.Time) Retention {
	s = s.Normalised()
	out := Retention{Serial: r.Serial, Years: s.RetentionYears}

	completed, ok := CompletionDate(r)
	if !ok {
		out.Basis = "no usable procedure date"
		out.Note = "This entry has no procedure date, so there is no date to measure retention from. The date the procedure was carried out is item 13 of the form and it is what the retention window runs from."
		return out
	}
	out.CompletedOn = FormatDay(completed)
	out.Basis = "date the procedure was completed (item 13)"

	until := AddYearsClamped(completed, s.RetentionYears)
	out.RetainUntil = FormatDay(until)
	out.DaysRemaining = DaysBetween(dayOf(asOf), until)

	if r.ProceedingsPending {
		out.OpenEnded = true
		out.Note = fmt.Sprintf("Proceedings are noted against this entry. Keep it, and every paper filed with it, until those proceedings are finally disposed of or until %s, whichever is later. %s", out.RetainUntil, RetentionProviso+".")
		return out
	}

	out.Releasable = out.DaysRemaining < 0
	if out.Releasable {
		out.Note = fmt.Sprintf("The %d-year window closed on %s. Check that no proceedings touch this entry and confirm with the Appropriate Authority before anything is destroyed.", s.RetentionYears, out.RetainUntil)
	} else {
		out.Note = fmt.Sprintf("Hold until %s inclusive — %d day(s) to go on a %d-year window.", out.RetainUntil, out.DaysRemaining, s.RetentionYears)
	}
	return out
}

// RetentionSummary counts a register's retention position in one pass.
type RetentionSummary struct {
	AsOf        string `json:"asOf"`
	Years       int    `json:"years"`
	Total       int    `json:"total"`
	Holding     int    `json:"holding"`
	Releasable  int    `json:"releasable"`
	OpenEnded   int    `json:"openEnded"`
	Undatable   int    `json:"undatable"`
	EarliestDue string `json:"earliestDue,omitempty"`
	Reference   string `json:"reference"`
}

// SummariseRetention returns the retention position of a whole register.
func SummariseRetention(records []Record, s Settings, asOf time.Time) RetentionSummary {
	s = s.Normalised()
	out := RetentionSummary{
		AsOf:      FormatDay(asOf),
		Years:     s.RetentionYears,
		Total:     len(records),
		Reference: RetentionReference,
	}
	var earliest time.Time
	for _, r := range records {
		st := RetentionOn(r, s, asOf)
		switch {
		case st.OpenEnded:
			out.OpenEnded++
		case st.RetainUntil == "":
			out.Undatable++
		case st.Releasable:
			out.Releasable++
		default:
			out.Holding++
			if d, err := ParseDay(st.RetainUntil); err == nil {
				if earliest.IsZero() || d.Before(earliest) {
					earliest = d
				}
			}
		}
	}
	if !earliest.IsZero() {
		out.EarliestDue = FormatDay(earliest)
	}
	return out
}
