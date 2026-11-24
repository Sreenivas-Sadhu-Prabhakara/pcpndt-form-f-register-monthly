package formf

import (
	"fmt"
	"strings"
	"time"
)

// Completeness checking, which is the whole product.
//
// The reason this tool exists is in the proviso to section 4(3) of the Act:
// any deficiency or inaccuracy in the record kept of an ultrasound on a
// pregnant woman amounts to a contravention of section 5 or section 6 unless
// the person who conducted it proves otherwise. A blank field is not a
// clerical slip that gets a warning letter. It shifts the burden of proof onto
// the sonologist, and the exposure behind it runs from prosecution under
// section 23 through suspension of the centre's registration to the seizure
// and sealing of records and equipment.
//
// The form is long — twenty-seven items across four sections — it is filled in
// between patients, and the field that gets skipped is never the same one
// twice.
//
// So Validate does not stop at the first blank. It walks the entire field set
// and names every mandatory item that is empty, in the order the form prints
// them, in one pass. A checker that reported one problem at a time would turn
// a two-minute correction into eleven trips back to the desk, and the person
// filling the form needs to see the whole shortfall at once to know whether
// the answers can still be found.
//
// On top of the blanks it checks what can only be wrong in combination: a
// declaration dated after the procedure it was supposed to precede, a
// gestational age that does not follow from the stated last menstrual period, a
// referral that is neither named nor a self-referral, an entry signed by
// somebody other than the person who performed the scan.

// IssueKind separates the three different things that can be wrong.
type IssueKind string

const (
	// IssueMissing is a mandatory item left blank.
	IssueMissing IssueKind = "missing"
	// IssueInconsistent is a set of answers that cannot all be true.
	IssueInconsistent IssueKind = "inconsistent"
	// IssueRefused is free text carrying a checked term. The entry is not
	// stored until the text is changed; see screening.go.
	IssueRefused IssueKind = "refused"
)

// Issue is one thing wrong with an entry, stated so that it can be acted on
// without looking anything up.
type Issue struct {
	Kind    IssueKind `json:"kind"`
	Field   string    `json:"field"`
	Item    string    `json:"item,omitempty"`
	Label   string    `json:"label,omitempty"`
	Message string    `json:"message"`
	// Terms names the terms found, for a refusal only.
	Terms []string `json:"terms,omitempty"`
}

// Validation is everything wrong with one entry, found in one pass.
type Validation struct {
	Serial    int     `json:"serial"`
	Issues    []Issue `json:"issues"`
	Mandatory int     `json:"mandatoryFields"`
	Filled    int     `json:"filledMandatoryFields"`
	// Summary states the position in one line. It is a field rather than
	// a method so that it travels with the rest over the wire, and every
	// surface says the same sentence.
	Summary string `json:"summary"`
}

// Validate checks one entry against the whole field set and returns every
// problem at once. It is pure: it reads nothing but the record.
//
// The checks that need to know today's date — a procedure dated in the future,
// a signature dated in the future — are in ValidateOn.
func Validate(r Record) Validation { return ValidateOn(r, time.Time{}) }

// ValidateOn is Validate with a reference day, which adds the checks that need
// to know when "now" is. Passing the zero time skips exactly those and nothing
// else.
func ValidateOn(r Record, asOf time.Time) Validation {
	v := Validation{Serial: r.Serial}

	// Every mandatory blank, in the order the paperwork prints the items.
	for _, f := range catalogue {
		if !f.RequiredFor(r) {
			continue
		}
		v.Mandatory++
		if f.filled(r) {
			v.Filled++
			continue
		}
		v.Issues = append(v.Issues, Issue{
			Kind:    IssueMissing,
			Field:   f.Key,
			Item:    f.Item,
			Label:   f.Label,
			Message: f.blankMessage(),
		})
	}

	v.Issues = append(v.Issues, consistencyIssues(r, asOf)...)
	v.Issues = append(v.Issues, screeningIssues(r)...)
	v.Summary = summarise(v)
	return v
}

// Complete reports whether the entry is a complete Form F: no blanks, no
// contradictions, no refused text.
func (v Validation) Complete() bool { return len(v.Issues) == 0 }

// Missing returns every mandatory item left blank, in the form's own order.
func (v Validation) Missing() []Issue { return v.of(IssueMissing) }

// Inconsistent returns every set of answers that cannot all be true.
func (v Validation) Inconsistent() []Issue { return v.of(IssueInconsistent) }

// Refused returns every free-text field carrying a checked term.
func (v Validation) Refused() []Issue { return v.of(IssueRefused) }

func (v Validation) of(k IssueKind) []Issue {
	var out []Issue
	for _, i := range v.Issues {
		if i.Kind == k {
			out = append(out, i)
		}
	}
	return out
}

// MissingFields returns just the keys of the blank mandatory items, which is
// what an interface needs in order to mark them.
func (v Validation) MissingFields() []string {
	out := make([]string, 0, len(v.Issues))
	for _, i := range v.Issues {
		if i.Kind == IssueMissing {
			out = append(out, i.Field)
		}
	}
	return out
}

// Storable reports whether the entry may be written to the register at all.
// A blank or a contradiction can be corrected later and the entry still has to
// exist; text carrying a checked term is never stored.
func (v Validation) Storable() bool { return len(v.Refused()) == 0 }

// summarise states the position in one line.
func summarise(v Validation) string {
	if v.Complete() {
		return fmt.Sprintf("Complete: all %d applicable items are filled in.", v.Mandatory)
	}
	var parts []string
	if n := len(v.Missing()); n > 0 {
		parts = append(parts, fmt.Sprintf("%d mandatory item%s blank", n, plural(n, "", "s")))
	}
	if n := len(v.Inconsistent()); n > 0 {
		parts = append(parts, fmt.Sprintf("%d answer%s that cannot be right together", n, plural(n, "", "s")))
	}
	if n := len(v.Refused()); n > 0 {
		parts = append(parts, fmt.Sprintf("%d field%s refused", n, plural(n, "", "s")))
	}
	return "Incomplete: " + strings.Join(parts, ", ") + "."
}

// ---------------------------------------------------------- cross-field rules

// consistencyIssues holds the rules no single field can be judged by. Each one
// is a separate statement so that a test can aim at it directly.
func consistencyIssues(r Record, asOf time.Time) []Issue {
	var out []Issue
	add := func(key, msg string) {
		f := fieldByKey(key)
		out = append(out, Issue{Kind: IssueInconsistent, Field: key, Item: f.Item, Label: f.Label, Message: msg})
	}
	// badDate reports a value that is present but is not a date.
	badDate := func(key, value string) {
		add(key, fmt.Sprintf("%q is not a calendar date. Use YYYY-MM-DD.", value))
	}

	procedure, perr := ParseDay(r.ProcedureDate)
	procedureOK := perr == nil
	if r.ProcedureDate != "" && !procedureOK {
		badDate("procedureDate", r.ProcedureDate)
	}

	// She reports, and then the procedure is done. Rule 9(1) asks for the
	// date she first reported, and it cannot fall after the scan.
	if first, err := ParseDay(r.FirstReportedOn); err == nil {
		if procedureOK && first.After(procedure) {
			add("firstReportedOn", fmt.Sprintf("She is recorded as first reporting on %s, after the procedure on %s. One of the two dates is wrong.", r.FirstReportedOn, r.ProcedureDate))
		}
		if procedureOK && DaysBetween(first, procedure) > 300 {
			add("firstReportedOn", fmt.Sprintf("She is recorded as first reporting on %s, %d days before the procedure. Check the year.", r.FirstReportedOn, DaysBetween(first, procedure)))
		}
	} else if r.FirstReportedOn != "" {
		badDate("firstReportedOn", r.FirstReportedOn)
	}

	// Her declaration is obtained before the procedure, not after it.
	if decl, err := ParseDay(r.WomanDeclarationDate); err == nil {
		if procedureOK && decl.After(procedure) {
			add("womanDeclarationDate", fmt.Sprintf("Her declaration is dated %s, after the procedure on %s. It is obtained before the procedure, so one of the two dates is wrong.", r.WomanDeclarationDate, r.ProcedureDate))
		}
		if procedureOK && DaysBetween(decl, procedure) > 30 {
			add("womanDeclarationDate", fmt.Sprintf("Her declaration is dated %s, %d days before the procedure on %s. The declaration is given for this procedure, not months ahead of it.", r.WomanDeclarationDate, DaysBetween(decl, procedure), r.ProcedureDate))
		}
	} else if r.WomanDeclarationDate != "" {
		badDate("womanDeclarationDate", r.WomanDeclarationDate)
	}

	// An entry is signed on the day of the procedure or after it, and not
	// much after it.
	if signed, err := ParseDay(r.SignedOn); err == nil {
		if procedureOK && signed.Before(procedure) {
			add("signedOn", fmt.Sprintf("The entry is signed %s but the procedure is dated %s. An entry cannot be signed before the procedure it records.", r.SignedOn, r.ProcedureDate))
		}
		if procedureOK && DaysBetween(procedure, signed) > 7 {
			add("signedOn", fmt.Sprintf("The entry is signed %s, %d days after the procedure on %s. The record is completed at the time; a long gap is itself a finding.", r.SignedOn, DaysBetween(procedure, signed), r.ProcedureDate))
		}
	} else if r.SignedOn != "" {
		badDate("signedOn", r.SignedOn)
	}

	// Rule 10(1A) puts the conductor's declaration on the report of the
	// procedure, so it is dated on or after the procedure.
	if dd, err := ParseDay(r.DoctorDeclarationDate); err == nil {
		if procedureOK && dd.Before(procedure) {
			add("doctorDeclarationDate", fmt.Sprintf("The declaration by the person who conducted the procedure is dated %s, before the procedure on %s.", r.DoctorDeclarationDate, r.ProcedureDate))
		}
		if procedureOK && DaysBetween(procedure, dd) > 7 {
			add("doctorDeclarationDate", fmt.Sprintf("The declaration by the person who conducted the procedure is dated %s, %d days after it. It belongs on the report of the procedure.", r.DoctorDeclarationDate, DaysBetween(procedure, dd)))
		}
	} else if r.DoctorDeclarationDate != "" {
		badDate("doctorDeclarationDate", r.DoctorDeclarationDate)
	}

	// The last menstrual period precedes the procedure, and the weeks of
	// pregnancy have to follow from it. Item 8 accepts either; recording
	// both lets each check the other, which is the point.
	if lmp, err := ParseDay(r.LMP); err == nil {
		if procedureOK {
			days := DaysBetween(lmp, procedure)
			switch {
			case days < 0:
				add("lmp", fmt.Sprintf("The last menstrual period is dated %s, after the procedure on %s.", r.LMP, r.ProcedureDate))
			case days > 320:
				add("lmp", fmt.Sprintf("The last menstrual period is dated %s, %d days before the procedure. Check the year.", r.LMP, days))
			default:
				if r.GestationWeeks > 0 {
					implied := days / 7
					if abs(implied-r.GestationWeeks) > 2 {
						add("gestationWeeks", fmt.Sprintf("%d weeks is recorded, but a last menstrual period of %s puts the pregnancy at about %d weeks on the day of the procedure. Check which of the two is right.", r.GestationWeeks, r.LMP, implied))
					}
				}
			}
		}
	} else if r.LMP != "" {
		badDate("lmp", r.LMP)
	}

	if r.GestationWeeks < 0 || r.GestationWeeks > 44 {
		add("gestationWeeks", fmt.Sprintf("%d weeks is not a possible gestational age.", r.GestationWeeks))
	}
	if r.Age != 0 && (r.Age < 10 || r.Age > 70) {
		add("age", fmt.Sprintf("An age of %d is outside any plausible range for this record. Check the figure.", r.Age))
	}
	if r.LivingChildren != nil && (*r.LivingChildren < 0 || *r.LivingChildren > 20) {
		add("livingChildren", fmt.Sprintf("%d is not a plausible number of living children.", *r.LivingChildren))
	}
	if r.LivingChildren != nil && *r.LivingChildren == 0 && trimmed(r.ChildAges) {
		add("childAges", "The total number of living children is nil but ages have been entered against item 4. One of the two is wrong.")
	}

	// A referral is either named or it is a self-referral, and the form is
	// explicit that self-referral has a narrow meaning.
	if r.SelfReferral && trimmed(r.ReferrerName) {
		add("selfReferral", fmt.Sprintf("The entry is marked a self-referral and also names %q as the referring doctor. Only one of the two can be true, and a self-referral means the person conducting the procedure referred her.", strings.TrimSpace(r.ReferrerName)))
	}

	// Section C territory. An ultrasound clinic writing an invasive
	// technique into item 11 has either used a technique its registration
	// does not cover or is filling in the wrong section.
	if found := MentionsInvasiveTechnique(r.ProcedureOther); len(found) > 0 {
		add("procedureOther", fmt.Sprintf("%q names an invasive pre-natal diagnostic technique (%s). Those belong to item 21 in Section C of the form, which this register does not keep, and a centre registered for ultrasound alone is not registered to perform them. Confirm the centre's certificate of registration covers it, and complete Section C on the form itself.", strings.TrimSpace(r.ProcedureOther), strings.Join(found, ", ")))
	}

	// Item 16 answered both ways at once.
	if r.MTPIndication == MTPNone && trimmed(r.MTPDetail) {
		add("mtpDetail", "Item 16 records no indication for a termination, but details have been entered against it. One of the two is wrong.")
	}

	// A result conveyed needs both halves of item 15.
	if trimmed(r.ConveyedOn) && !trimmed(r.ConveyedTo) {
		add("conveyedTo", "Item 15 has a date for conveying the result but nobody is named as having received it.")
	}
	if trimmed(r.ConveyedTo) && !trimmed(r.ConveyedOn) {
		add("conveyedOn", "Item 15 names who the result was conveyed to but gives no date.")
	}
	if conveyed, err := ParseDay(r.ConveyedOn); err == nil {
		if procedureOK && conveyed.Before(procedure) {
			add("conveyedOn", fmt.Sprintf("The result is recorded as conveyed on %s, before the procedure on %s.", r.ConveyedOn, r.ProcedureDate))
		}
	} else if trimmed(r.ConveyedOn) {
		badDate("conveyedOn", r.ConveyedOn)
	}

	// A thumb impression is attested at the time it is given.
	if att, err := ParseDay(r.AttestedOn); err == nil {
		if decl, derr := ParseDay(r.WomanDeclarationDate); derr == nil && att.Before(decl) {
			add("attestedOn", fmt.Sprintf("The thumb impression is attested on %s, before the declaration it attests, dated %s.", r.AttestedOn, r.WomanDeclarationDate))
		}
	} else if trimmed(r.AttestedOn) {
		badDate("attestedOn", r.AttestedOn)
	}
	if !r.ThumbImpression && (trimmed(r.IdentifierName) || trimmed(r.AttestedBy)) {
		add("thumbImpression", "The declaration is not recorded as signed with a thumb impression, but the identifying and attesting particulars have been filled in. Those are asked for only where a thumb impression was given.")
	}

	// The signature block belongs to the person who performed the scan.
	if trimmed(r.SignatoryName) && trimmed(r.ConductedByName) && !sameName(r.SignatoryName, r.ConductedByName) {
		add("signatoryName", fmt.Sprintf("Item 9 records %q as performing the procedure but the entry is signed by %q. The Section B block is signed by the person who performed it; if someone else signed, the entry needs explaining.", strings.TrimSpace(r.ConductedByName), strings.TrimSpace(r.SignatoryName)))
	}

	// Wording that is not on item 10's list at all, which happens when a
	// data file has been edited by hand.
	for _, ind := range r.Indications {
		if !trimmed(ind) || IsListedIndication(ind) || ind == IndOther {
			continue
		}
		add("indications", fmt.Sprintf("%q is not one of the indications item 10 prints, and not the entry this register adds for anything outside that list. Choose from the list and put the wording in the box beside it.", ind))
	}

	// Dates that have not happened yet.
	if !asOf.IsZero() {
		today := dayOf(asOf)
		for _, c := range []struct{ key, value string }{
			{"firstReportedOn", r.FirstReportedOn},
			{"womanDeclarationDate", r.WomanDeclarationDate},
			{"procedureDate", r.ProcedureDate},
			{"conveyedOn", r.ConveyedOn},
			{"signedOn", r.SignedOn},
			{"doctorDeclarationDate", r.DoctorDeclarationDate},
			{"attestedOn", r.AttestedOn},
			{"lmp", r.LMP},
		} {
			d, err := ParseDay(c.value)
			if err != nil || !d.After(today) {
				continue
			}
			add(c.key, fmt.Sprintf("%s is dated %s, which is in the future.", fieldByKey(c.key).Label, c.value))
		}
	}
	return out
}

// sameName compares two names loosely enough to accept "Dr M. Rao" against
// "Dr. M Rao", and strictly enough to notice a different person.
func sameName(a, b string) bool {
	norm := func(s string) string {
		s = strings.ToLower(s)
		var b strings.Builder
		for _, r := range s {
			if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
				b.WriteRune(r)
			}
		}
		out := b.String()
		return strings.TrimPrefix(out, "dr")
	}
	return norm(a) == norm(b)
}

// screeningIssues refuses any free-text field carrying a checked term. The
// refusal names the field and every term found in it, so that the correction
// needs no guesswork. See screening.go for what is checked and why.
func screeningIssues(r Record) []Issue {
	var out []Issue
	for _, f := range catalogue {
		if !f.Screened {
			continue
		}
		terms := ScreenText(f.render(r))
		if len(terms) == 0 {
			continue
		}
		out = append(out, Issue{
			Kind:  IssueRefused,
			Field: f.Key,
			Item:  f.Item,
			Label: f.Label,
			Terms: terms,
			Message: fmt.Sprintf("%s cannot be stored as written: it contains %s. This register holds no record of what the Act prohibits, in any field. Rewrite it without %s and the entry will save.",
				f.Label, quoteList(terms), plural(len(terms), "that word", "those words")),
		})
	}
	return out
}

func quoteList(terms []string) string {
	parts := make([]string, len(terms))
	for i, t := range terms {
		parts[i] = fmt.Sprintf("%q", t)
	}
	switch len(parts) {
	case 1:
		return parts[0]
	case 2:
		return parts[0] + " and " + parts[1]
	default:
		return strings.Join(parts[:len(parts)-1], ", ") + " and " + parts[len(parts)-1]
	}
}

func abs(n int) int {
	if n < 0 {
		return -n
	}
	return n
}
