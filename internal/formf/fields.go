package formf

import (
	"strconv"
	"strings"
)

// The prescribed field set.
//
// This catalogue is the single description of what an entry holds, and
// everything else is driven by it: the completeness check walks it in order,
// the printed pages take their columns and their item numbers from it, the
// interface builds the form from it, and the test that asserts this register
// has nowhere to put what the Act prohibits reads it rather than trusting a
// comment. Add a field anywhere else and it appears on no page; add it here
// and it appears in the right place with the right number.
//
// Item numbers are the numbers printed on the Form F substituted by G.S.R.
// 77(E) of 31 January 2014, so that a blank reported by the checker can be
// found on the paper form without counting down the page. Two items carry a
// rule number instead of a form number, because they are prescribed by rule
// 9(1) rather than by the form, and three carry neither, because they are the
// register's own and are marked as such.

// FieldKind tells an interface how to render an item.
type FieldKind string

const (
	KindText     FieldKind = "text"
	KindLongText FieldKind = "longtext"
	KindDate     FieldKind = "date"
	KindNumber   FieldKind = "number"
	KindChoice   FieldKind = "choice"
	KindMulti    FieldKind = "multi"
	KindCheck    FieldKind = "check"
)

// Field groups, in the order the form works through them.
const (
	GroupRule9Register = "Rule 9(1) register"
	GroupSectionA      = "Section A — for all diagnostic procedures and tests"
	GroupSectionB      = "Section B — non-invasive procedures only"
	GroupSignature     = "Section B — signature block"
	GroupSectionD      = "Section D — declarations"
	GroupLocal         = "Kept by this register, not asked for by the form"
)

// Field is one item as this register holds it.
type Field struct {
	// Key is the stable name used in the data file and the interface.
	Key string `json:"key"`
	// Item is the number as printed on Form F, or the rule that
	// prescribes the item where the form does not. It is empty for the
	// register's own fields.
	Item string `json:"item,omitempty"`
	// Group is the part of the form the item belongs to.
	Group string `json:"group"`
	// Label is what the interface and the printed Form F face show.
	Label string `json:"label"`
	// Column is the short heading used in the printed working index.
	Column string `json:"column"`
	// Help says what the item wants. It is repeated in the message when
	// the item is reported blank, so it is written to be read there.
	Help string `json:"help,omitempty"`
	// Kind tells the interface how to render the item.
	Kind FieldKind `json:"kind"`
	// Choices lists the permitted values of a choice or multi item.
	Choices []string `json:"choices,omitempty"`
	// Mandatory marks an item that is always required.
	Mandatory bool `json:"mandatory"`
	// Conditional marks an item required only in some circumstances, with
	// When saying which.
	Conditional bool   `json:"conditional,omitempty"`
	When        string `json:"when,omitempty"`
	// Statutory is false for the three fields this register keeps that the
	// form and the rules do not ask for.
	Statutory bool `json:"statutory"`
	// Screened is true where the value is free text checked before the
	// entry is stored.
	Screened bool `json:"screened,omitempty"`

	filled       func(Record) bool
	render       func(Record) string
	requiredWhen func(Record) bool
}

// RequiredFor reports whether this item has to be filled in on this record.
func (f Field) RequiredFor(r Record) bool {
	if f.Mandatory {
		return true
	}
	return f.requiredWhen != nil && f.requiredWhen(r)
}

// Value returns the item's value on a record, rendered for printing.
func (f Field) Value(r Record) string { return f.render(r) }

// Filled reports whether the item holds an answer on this record.
func (f Field) Filled(r Record) bool { return f.filled(r) }

func (f Field) blankMessage() string {
	var b strings.Builder
	switch {
	case f.Item == "":
		b.WriteString(f.Label)
	case strings.HasPrefix(f.Item, "Rule"):
		b.WriteString(f.Item)
		b.WriteString(" — ")
		b.WriteString(f.Label)
	case strings.HasPrefix(f.Item, "Section"):
		b.WriteString("Form F, ")
		b.WriteString(f.Item)
		b.WriteString(" — ")
		b.WriteString(f.Label)
	default:
		b.WriteString("Form F item ")
		b.WriteString(f.Item)
		b.WriteString(" — ")
		b.WriteString(f.Label)
	}
	b.WriteString(" — is blank.")
	if f.Help != "" {
		b.WriteString(" ")
		b.WriteString(f.Help)
	}
	return b.String()
}

// Fields returns the prescribed field set in the form's own order. The slice
// is a copy: a caller cannot reorder or edit the register's field set.
func Fields() []Field {
	out := make([]Field, len(catalogue))
	copy(out, catalogue)
	return out
}

// FieldByKey returns one item by key, and whether it exists.
func FieldByKey(key string) (Field, bool) {
	for _, f := range catalogue {
		if f.Key == key {
			return f, true
		}
	}
	return Field{}, false
}

// fieldByKey is the internal form, returning a zero Field for an unknown key
// so that message building never has to branch.
func fieldByKey(key string) Field {
	f, _ := FieldByKey(key)
	return f
}

// ItemNotStored describes something the prescribed paperwork asks for that
// this register deliberately does not hold.
type ItemNotStored struct {
	Item   string `json:"item"`
	Asks   string `json:"asks"`
	Reason string `json:"reason"`
	Where  string `json:"where"`
}

// ItemsNotStored lists, openly, what this register leaves out and why. A tool
// that silently dropped part of a statutory form would be worse than one that
// never offered the form at all, so each omission is published here, printed
// at the foot of every page this tool produces, and shown in the interface
// beside the form itself.
func ItemsNotStored() []ItemNotStored {
	return []ItemNotStored{
		{
			Item: "4(a) and 4(b)",
			Asks: "Item 4 asks for the total number of living children and then splits that total two ways, giving the age of each child in each part.",
			Reason: "This register holds the total and the ages and stops there. It keeps no field that splits a household's children the way item 4 splits them — " +
				"not a column, not a count, not a coded value. That split is the thing the Act exists to stop anyone acting on, and a clinic computer holding it for " +
				"every household in the district would be a list of precisely the wrong kind.",
			Where: "It is completed on the signed Form F the centre keeps, which is the document the Appropriate Authority inspects.",
		},
		{
			Item:   "Section D, thumb impression block",
			Asks:   "Where the declaration is signed with a thumb impression, the form asks for the identifying person's name, age and one further particular.",
			Reason: "The name, age, relation, address and contact number are held here. The further particular is not, for the same reason as item 4.",
			Where:  "It is completed on the signed form.",
		},
		{
			Item:   "Section C, items 17 to 27",
			Asks:   "Section C covers the invasive pre-natal diagnostic procedures — amniocentesis, chorionic villi aspiration, foetal biopsy, cordocentesis and the rest — together with the genetic indications and the Form G consent for them.",
			Reason: "This register is for ultrasound clinics and imaging centres, which are not registered to perform any of them. Offering the fields would invite an entry nobody at such a centre is entitled to make.",
			Where:  "A centre that does hold that registration completes Section C on the form itself; this tool does not cover it and does not claim to.",
		},
		{
			Item:   "Item 11, the note printed beneath it",
			Asks:   "Item 11 carries a note of its own about what an ultrasound is not performed for.",
			Reason: "That note is on the form and the form is where it belongs. This register does not restate it, and nothing anywhere in this register provides for the thing it refers to.",
			Where:  "Read it on the form.",
		},
	}
}

// PolicyStatement is the standing statement of what this register will not
// hold. It is shown in the interface, printed on every page and reproduced in
// the documentation, and it is a constant rather than a line in a template so
// that it says the same thing everywhere it appears.
const PolicyStatement = "This register does not record, infer, derive, display, print or export the sex of a foetus. " +
	"There is no field for it, no coded value that stands for it, no note that carries it and no report that totals it, and there never will be. " +
	"Free-text clinical fields are checked before an entry is stored, and an entry carrying such a term is refused with the term named — not saved with a warning. " +
	"The Act this register serves exists to stop sex selection and the destruction of unborn girls that follows from it, and a tool built around its paperwork has to be built so that it cannot be turned to that purpose. " +
	"That is also why one part of item 4 of the form is not held here at all."

// catalogue is the field set itself. The order is the order of the paperwork,
// and it is also the column order of the printed working index.
var catalogue = []Field{
	// ---- rule 9(1): the serially numbered register ----
	{
		Key: "serial", Item: "Rule 9(1)", Group: GroupRule9Register,
		Label: "Serial number in the register", Column: "Sl.",
		Help:      "Assigned by this register, never typed in. Rule 9(1) requires the register to run in serial order; a missing number reads as a removed record.",
		Kind:      KindNumber,
		Mandatory: true, Statutory: true,
		filled: func(r Record) bool { return r.Serial > 0 },
		render: func(r Record) string { return strconv.Itoa(r.Serial) },
	},
	{
		Key: "firstReportedOn", Item: "Rule 9(1)", Group: GroupRule9Register,
		Label: "Date she first reported", Column: "First reported",
		Help:      "Rule 9(1) requires the date on which she first reported for the procedure. It is often the same day as the scan and it is not always.",
		Kind:      KindDate,
		Mandatory: true, Statutory: true,
		filled: func(r Record) bool { _, err := ParseDay(r.FirstReportedOn); return err == nil },
		render: func(r Record) string { return r.FirstReportedOn },
	},

	// ---- Section A: for all diagnostic procedures and tests ----
	{
		Key: "patientName", Item: "3", Group: GroupSectionA,
		Label: "Her name", Column: "Name",
		Help:      "Her full name, as on the identification seen at the visit. Rule 9(1) requires it in the register as well.",
		Kind:      KindText,
		Mandatory: true, Statutory: true,
		filled: func(r Record) bool { return trimmed(r.PatientName) },
		render: func(r Record) string { return r.PatientName },
	},
	{
		Key: "age", Item: "3", Group: GroupSectionA,
		Label: "Her age", Column: "Age",
		Help:      "In completed years. Item 3 asks for the age beside the name, and an inspection checks the pair together.",
		Kind:      KindNumber,
		Mandatory: true, Statutory: true,
		filled: func(r Record) bool { return r.Age > 0 },
		render: func(r Record) string { return intOrBlank(r.Age) },
	},
	{
		Key: "livingChildren", Item: "4", Group: GroupSectionA,
		Label: "Total number of living children", Column: "Children",
		Help:      "The total. Enter 0 where there are none — an empty box is an unanswered item, not a zero. Item 4 then splits that total in a way this register does not hold; see what this register does not store.",
		Kind:      KindNumber,
		Mandatory: true, Statutory: true,
		filled: func(r Record) bool { return r.LivingChildren != nil && *r.LivingChildren >= 0 },
		render: func(r Record) string {
			if r.LivingChildren == nil {
				return ""
			}
			return strconv.Itoa(*r.LivingChildren)
		},
	},
	{
		Key: "childAges", Item: "4", Group: GroupSectionA,
		Label: "Age of each living child", Column: "Ages",
		Help:        "Item 4 asks for the age of each child, in years or months. Write them as they were given, for example \"6 y, 3 y, 11 m\".",
		Kind:        KindText,
		Conditional: true, When: "where there is at least one living child", Statutory: true,
		Screened:     true,
		filled:       func(r Record) bool { return trimmed(r.ChildAges) },
		render:       func(r Record) string { return r.ChildAges },
		requiredWhen: func(r Record) bool { return r.LivingChildren != nil && *r.LivingChildren > 0 },
	},
	{
		Key: "relativeName", Item: "5", Group: GroupSectionA,
		Label: "Husband's, wife's, father's or mother's name", Column: "Relative",
		Help:      "Item 5 as the 2014 form words it. Rule 9(1) requires the spouse's or father's name in the register too.",
		Kind:      KindText,
		Mandatory: true, Statutory: true,
		filled: func(r Record) bool { return trimmed(r.RelativeName) },
		render: func(r Record) string { return r.RelativeName },
	},
	{
		Key: "address", Item: "6", Group: GroupSectionA,
		Label: "Her full postal address", Column: "Address",
		Help:      "The full postal address. Rule 9(1) requires it in the register, and an address that cannot be traced is a finding on its own.",
		Kind:      KindLongText,
		Mandatory: true, Statutory: true,
		filled: func(r Record) bool { return trimmed(r.Address) },
		render: func(r Record) string { return r.Address },
	},
	{
		Key: "phone", Item: "6", Group: GroupSectionA,
		Label: "Her contact number", Column: "Phone",
		Help:      "Item 6 asks for it where there is one.",
		Kind:      KindText,
		Statutory: true,
		filled:    func(r Record) bool { return trimmed(r.Phone) },
		render:    func(r Record) string { return r.Phone },
	},
	{
		Key: "selfReferral", Item: "7(b)", Group: GroupSectionA,
		Label: "Self-referral", Column: "Self-ref.",
		Help: "Tick this only where the gynaecologist, radiologist or registered medical practitioner conducting the procedure is the one who referred her. " +
			"The form states on its face that self-referral does not mean a client coming to a clinic and asking for the test, and does not mean a relative asking for it either. " +
			"If nobody referred her, that is not a self-referral: it is a procedure without a referral.",
		Kind:      KindCheck,
		Statutory: true,
		filled:    func(r Record) bool { return r.SelfReferral },
		render:    func(r Record) string { return yesNo(r.SelfReferral) },
	},
	{
		Key: "referrerName", Item: "7(a)", Group: GroupSectionA,
		Label: "Referred by — name", Column: "Referred by",
		Help:        "The full name of the doctor or genetic counselling centre that referred her.",
		Kind:        KindText,
		Conditional: true, When: "unless the entry is a self-referral", Statutory: true,
		filled:       func(r Record) bool { return trimmed(r.ReferrerName) },
		render:       func(r Record) string { return r.ReferrerName },
		requiredWhen: func(r Record) bool { return !r.SelfReferral },
	},
	{
		Key: "referrerAddress", Item: "7(a)", Group: GroupSectionA,
		Label: "Referred by — address", Column: "Referrer address",
		Help:        "Item 7(a) asks for the address as well as the name.",
		Kind:        KindLongText,
		Conditional: true, When: "unless the entry is a self-referral", Statutory: true,
		filled:       func(r Record) bool { return trimmed(r.ReferrerAddress) },
		render:       func(r Record) string { return r.ReferrerAddress },
		requiredWhen: func(r Record) bool { return !r.SelfReferral },
	},
	{
		Key: "referrerQualification", Item: "7(a)", Group: GroupSectionA,
		Label: "Referred by — qualification", Column: "Referrer qualification",
		Help:        "The Ministry's inspection checklist for item 7 looks for the referring doctor's qualification alongside the name and address.",
		Kind:        KindText,
		Conditional: true, When: "unless the entry is a self-referral", Statutory: true,
		filled:       func(r Record) bool { return trimmed(r.ReferrerQualificatn) },
		render:       func(r Record) string { return r.ReferrerQualificatn },
		requiredWhen: func(r Record) bool { return !r.SelfReferral },
	},
	{
		Key: "referralPapersHeld", Item: "7", Group: GroupSectionA,
		Label: "Referral slip or self-referral note filed with this entry", Column: "Papers filed",
		Help: "Item 7 says in two places that the paperwork is preserved with the Form F: the referral slip for 7(a), and for 7(b) the referral note with the indications " +
			"together with the patient's case papers. A missing referral slip is listed as a contravention in its own right. Tick this only when the paper is actually in the file.",
		Kind:      KindCheck,
		Mandatory: true, Statutory: true,
		filled: func(r Record) bool { return r.ReferralPapersHeld },
		render: func(r Record) string { return yesNo(r.ReferralPapersHeld) },
	},
	{
		Key: "lmp", Item: "8", Group: GroupSectionA,
		Label: "Last menstrual period", Column: "LMP",
		Help:      "The first day of the last menstrual period, as YYYY-MM-DD. Item 8 asks for this or the weeks of pregnancy; this register asks for both and checks one against the other.",
		Kind:      KindDate,
		Mandatory: true, Statutory: true,
		filled: func(r Record) bool { _, err := ParseDay(r.LMP); return err == nil },
		render: func(r Record) string { return r.LMP },
	},
	{
		Key: "gestationWeeks", Item: "8", Group: GroupSectionA,
		Label: "Weeks of pregnancy", Column: "Weeks",
		Help:      "Completed weeks on the day of the procedure. It has to follow from the last menstrual period.",
		Kind:      KindNumber,
		Mandatory: true, Statutory: true,
		filled: func(r Record) bool { return r.GestationWeeks > 0 },
		render: func(r Record) string { return intOrBlank(r.GestationWeeks) },
	},

	// ---- Section B: non-invasive procedures only ----
	{
		Key: "conductedByName", Item: "9", Group: GroupSectionB,
		Label: "Doctor performing the procedure", Column: "Performed by",
		Help:      "The name of the person who performed it, not the name of the centre. Only a person named on the centre's certificate of registration may perform it.",
		Kind:      KindText,
		Mandatory: true, Statutory: true,
		filled: func(r Record) bool { return trimmed(r.ConductedByName) },
		render: func(r Record) string { return r.ConductedByName },
	},
	{
		Key: "indications", Item: "10", Group: GroupSectionB,
		Label: "Indication for the procedure", Column: "Indication",
		Help:      "Tick every indication that applies. The form prints the list as a representative one and states that an ultrasound in pregnancy is performed only when indicated.",
		Kind:      KindMulti,
		Choices:   IndicationTexts(),
		Mandatory: true, Statutory: true,
		filled: func(r Record) bool { return nonEmpty(r.Indications) },
		render: func(r Record) string { return joinIndications(r.Indications) },
	},
	{
		Key: "indicationDetail", Item: "10", Group: GroupSectionB,
		Label: "Indication in words, against the referral", Column: "Indication detail",
		Help: "Item 10 asks for the indication to be specified with reference to the request made in the referral slip, or in the self-referral note. " +
			"A tick alone does not answer it: write what the referral asked for.",
		Kind:      KindLongText,
		Mandatory: true, Statutory: true,
		Screened: true,
		filled:   func(r Record) bool { return trimmed(r.IndicationDetail) },
		render:   func(r Record) string { return r.IndicationDetail },
	},
	{
		Key: "procedures", Item: "11", Group: GroupSectionB,
		Label: "Procedure carried out", Column: "Procedure",
		Help:      "Section B lists two: ultrasound, and anything else to be specified. The invasive techniques are item 21, in Section C, which this register does not cover.",
		Kind:      KindMulti,
		Choices:   Procedures(),
		Mandatory: true, Statutory: true,
		filled: func(r Record) bool { return nonEmpty(r.Procedures) },
		render: func(r Record) string { return strings.Join(r.Procedures, "; ") },
	},
	{
		Key: "procedureOther", Item: "11", Group: GroupSectionB,
		Label: "The other procedure, specified", Column: "Other procedure",
		Help:        "Name the technique used.",
		Kind:        KindText,
		Conditional: true, When: "where a procedure other than ultrasound was carried out", Statutory: true,
		Screened:     true,
		filled:       func(r Record) bool { return trimmed(r.ProcedureOther) },
		render:       func(r Record) string { return r.ProcedureOther },
		requiredWhen: func(r Record) bool { return hasChoice(r.Procedures, ProcOther) },
	},
	{
		Key: "womanDeclarationDate", Item: "12", Group: GroupSectionB,
		Label: "Date her declaration was obtained", Column: "Declaration date",
		Help:      "The declaration in Section D, signed by her, dated. It is obtained before the procedure and it is a separate thing from the written consent.",
		Kind:      KindDate,
		Mandatory: true, Statutory: true,
		filled: func(r Record) bool { _, err := ParseDay(r.WomanDeclarationDate); return err == nil },
		render: func(r Record) string { return r.WomanDeclarationDate },
	},
	{
		Key: "procedureDate", Item: "13", Group: GroupSectionB,
		Label: "Date the procedure was carried out", Column: "Procedure date",
		Help:      "As YYYY-MM-DD. This date places the entry in a month's report and it is the date the retention window runs from.",
		Kind:      KindDate,
		Mandatory: true, Statutory: true,
		filled: func(r Record) bool { _, err := ParseDay(r.ProcedureDate); return err == nil },
		render: func(r Record) string { return r.ProcedureDate },
	},
	{
		Key: "result", Item: "14", Group: GroupSectionB,
		Label: "Result of the procedure, in brief", Column: "Result",
		Help:      "Item 14 asks for the report in brief of the procedure carried out. This field is checked before the entry is stored; see what this register will not hold.",
		Kind:      KindLongText,
		Mandatory: true, Statutory: true,
		Screened: true,
		filled:   func(r Record) bool { return trimmed(r.Result) },
		render:   func(r Record) string { return r.Result },
	},
	{
		Key: "conveyedTo", Item: "15", Group: GroupSectionB,
		Label: "Result conveyed to", Column: "Conveyed to",
		Help:      "Item 15 asks who the result was conveyed to, and on what date.",
		Kind:      KindText,
		Statutory: true,
		filled:    func(r Record) bool { return trimmed(r.ConveyedTo) },
		render:    func(r Record) string { return r.ConveyedTo },
	},
	{
		Key: "conveyedOn", Item: "15", Group: GroupSectionB,
		Label: "Date the result was conveyed", Column: "Conveyed on",
		Help:      "As YYYY-MM-DD.",
		Kind:      KindDate,
		Statutory: true,
		filled:    func(r Record) bool { _, err := ParseDay(r.ConveyedOn); return err == nil },
		render:    func(r Record) string { return r.ConveyedOn },
	},
	{
		Key: "mtpIndication", Item: "16", Group: GroupSectionB,
		Label: "Indication for a medical termination from what was detected", Column: "MTP indication",
		Help:      "Item 16 asks whether anything found in the procedure gives an indication for a medical termination of pregnancy. Answer it in every case, including where there is none.",
		Kind:      KindChoice,
		Choices:   MTPAnswers(),
		Mandatory: true, Statutory: true,
		filled: func(r Record) bool { return r.MTPIndication == MTPNone || r.MTPIndication == MTPIndicated },
		render: func(r Record) string { return r.MTPIndication },
	},
	{
		Key: "mtpDetail", Item: "16", Group: GroupSectionB,
		Label: "The indication, stated", Column: "MTP detail",
		Help:        "State the abnormality detected and the indication it gives.",
		Kind:        KindLongText,
		Conditional: true, When: "where an indication for a termination is recorded", Statutory: true,
		Screened:     true,
		filled:       func(r Record) bool { return trimmed(r.MTPDetail) },
		render:       func(r Record) string { return r.MTPDetail },
		requiredWhen: func(r Record) bool { return r.MTPIndication == MTPIndicated },
	},

	// ---- Section B signature block ----
	{
		Key: "signatoryName", Item: "Section B signature", Group: GroupSignature,
		Label: "Name of the signatory", Column: "Signed by",
		Help:      "The block asks for the name, signature and registration number with seal of the gynaecologist, radiologist or registered medical practitioner who performed the procedure. It is the same person as item 9.",
		Kind:      KindText,
		Mandatory: true, Statutory: true,
		filled: func(r Record) bool { return trimmed(r.SignatoryName) },
		render: func(r Record) string { return r.SignatoryName },
	},
	{
		Key: "signatoryRegNo", Item: "Section B signature", Group: GroupSignature,
		Label: "Registration number of the signatory", Column: "Signatory reg. no.",
		Help:      "The registration number has to appear beside the signature. An entry signed without one is listed as an inspection finding.",
		Kind:      KindText,
		Mandatory: true, Statutory: true,
		filled: func(r Record) bool { return trimmed(r.SignatoryRegNo) },
		render: func(r Record) string { return r.SignatoryRegNo },
	},
	{
		Key: "sealAffixed", Item: "Section B signature", Group: GroupSignature,
		Label: "Seal affixed on the signed form", Column: "Seal",
		Help:      "The 2014 form asks for the seal as well as the name and the number. Tick this only once the seal is on the paper form.",
		Kind:      KindCheck,
		Mandatory: true, Statutory: true,
		filled: func(r Record) bool { return r.SealAffixed },
		render: func(r Record) string { return yesNo(r.SealAffixed) },
	},
	{
		Key: "signedOn", Item: "Section B signature", Group: GroupSignature,
		Label: "Date signed", Column: "Signed on",
		Help:      "The date beside the signature.",
		Kind:      KindDate,
		Mandatory: true, Statutory: true,
		filled: func(r Record) bool { _, err := ParseDay(r.SignedOn); return err == nil },
		render: func(r Record) string { return r.SignedOn },
	},
	{
		Key: "place", Item: "Section B signature", Group: GroupSignature,
		Label: "Place", Column: "Place",
		Help:      "The place of signing, as the block asks.",
		Kind:      KindText,
		Mandatory: true, Statutory: true,
		filled: func(r Record) bool { return trimmed(r.Place) },
		render: func(r Record) string { return r.Place },
	},

	// ---- Section D: the declarations ----
	{
		Key: "declarationLanguage", Item: "Section D", Group: GroupSectionD,
		Label: "Language her declaration was given in", Column: "Language",
		Help:      "The Ministry's inspection checklist asks whether her declaration was obtained in the language she understands. A declaration in a language she cannot read is not a declaration.",
		Kind:      KindText,
		Mandatory: true, Statutory: true,
		filled: func(r Record) bool { return trimmed(r.DeclarationLanguage) },
		render: func(r Record) string { return r.DeclarationLanguage },
	},
	{
		Key: "thumbImpression", Item: "Section D", Group: GroupSectionD,
		Label: "She signed with a thumb impression", Column: "Thumb impr.",
		Help:      "Tick this where she gave a thumb impression rather than a signature. The form then requires the identifying person's particulars and an attesting signature, and the register will ask for them.",
		Kind:      KindCheck,
		Statutory: true,
		filled:    func(r Record) bool { return r.ThumbImpression },
		render:    func(r Record) string { return yesNo(r.ThumbImpression) },
	},
	{
		Key: "identifierName", Item: "Section D", Group: GroupSectionD,
		Label: "Identified by — name", Column: "Identified by",
		Help:        "The name of the person who identified her.",
		Kind:        KindText,
		Conditional: true, When: "where she signed with a thumb impression", Statutory: true,
		filled:       func(r Record) bool { return trimmed(r.IdentifierName) },
		render:       func(r Record) string { return r.IdentifierName },
		requiredWhen: func(r Record) bool { return r.ThumbImpression },
	},
	{
		Key: "identifierAge", Item: "Section D", Group: GroupSectionD,
		Label: "Identified by — age", Column: "Identifier age",
		Help:        "In completed years.",
		Kind:        KindNumber,
		Conditional: true, When: "where she signed with a thumb impression", Statutory: true,
		filled:       func(r Record) bool { return r.IdentifierAge > 0 },
		render:       func(r Record) string { return intOrBlank(r.IdentifierAge) },
		requiredWhen: func(r Record) bool { return r.ThumbImpression },
	},
	{
		Key: "identifierRelation", Item: "Section D", Group: GroupSectionD,
		Label: "Identified by — relation, if any", Column: "Identifier relation",
		Help:      "The form asks for the relation if there is one.",
		Kind:      KindText,
		Statutory: true,
		filled:    func(r Record) bool { return trimmed(r.IdentifierRelation) },
		render:    func(r Record) string { return r.IdentifierRelation },
	},
	{
		Key: "identifierAddress", Item: "Section D", Group: GroupSectionD,
		Label: "Identified by — address and contact number", Column: "Identifier address",
		Help:        "The address and contact number of the identifying person.",
		Kind:        KindLongText,
		Conditional: true, When: "where she signed with a thumb impression", Statutory: true,
		filled:       func(r Record) bool { return trimmed(r.IdentifierAddress) },
		render:       func(r Record) string { return r.IdentifierAddress },
		requiredWhen: func(r Record) bool { return r.ThumbImpression },
	},
	{
		Key: "attestedBy", Item: "Section D", Group: GroupSectionD,
		Label: "Thumb impression attested by", Column: "Attested by",
		Help:        "The person who signed to attest the thumb impression. A thumb impression with nobody attesting it is an incomplete declaration.",
		Kind:        KindText,
		Conditional: true, When: "where she signed with a thumb impression", Statutory: true,
		filled:       func(r Record) bool { return trimmed(r.AttestedBy) },
		render:       func(r Record) string { return r.AttestedBy },
		requiredWhen: func(r Record) bool { return r.ThumbImpression },
	},
	{
		Key: "attestedOn", Item: "Section D", Group: GroupSectionD,
		Label: "Date of attestation", Column: "Attested on",
		Help:        "The date beside the attesting signature.",
		Kind:        KindDate,
		Conditional: true, When: "where she signed with a thumb impression", Statutory: true,
		filled:       func(r Record) bool { _, err := ParseDay(r.AttestedOn); return err == nil },
		render:       func(r Record) string { return r.AttestedOn },
		requiredWhen: func(r Record) bool { return r.ThumbImpression },
	},
	{
		Key: "doctorDeclarationSigned", Item: "Section D", Group: GroupSectionD,
		Label: "Declaration by the person conducting the procedure, signed", Column: "Doctor decl.",
		Help: "The second declaration in Section D, signed by the person who conducted the procedure, with the name in capitals and the registration number with seal. " +
			"It is required by rule 10(1A) and an unsigned one is listed as a contravention of that rule. An entry is not a Form F without it.",
		Kind:      KindCheck,
		Mandatory: true, Statutory: true,
		filled: func(r Record) bool { return r.DoctorDeclarationSigned },
		render: func(r Record) string { return yesNo(r.DoctorDeclarationSigned) },
	},
	{
		Key: "doctorDeclarationDate", Item: "Section D", Group: GroupSectionD,
		Label: "Date of that declaration", Column: "Doctor decl. date",
		Help:      "The inspection checklist asks for the declaration to carry the date, so the date is recorded here as well as on the paper.",
		Kind:      KindDate,
		Mandatory: true, Statutory: true,
		filled: func(r Record) bool { _, err := ParseDay(r.DoctorDeclarationDate); return err == nil },
		render: func(r Record) string { return r.DoctorDeclarationDate },
	},

	// ---- kept by this register ----
	{
		Key: "equipment", Group: GroupLocal,
		Label: "Machine used", Column: "Machine",
		Help:      "Not asked for by the form. The register keeps it so that a month can be reconciled against one machine's own log, which is how an entry that was never written gets found.",
		Kind:      KindText,
		Statutory: false,
		filled:    func(r Record) bool { return trimmed(r.Equipment) },
		render:    func(r Record) string { return r.Equipment },
	},
	{
		Key: "proceedingsPending", Group: GroupLocal,
		Label: "Proceedings noted against this entry", Column: "Proceedings",
		Help:      "Not asked for by the form. Tick it where legal proceedings touch this record: retention then runs until those proceedings are finally disposed of or until the ordinary window expires, whichever is later, and this register will never show the entry as releasable.",
		Kind:      KindCheck,
		Statutory: false,
		filled:    func(r Record) bool { return r.ProceedingsPending },
		render:    func(r Record) string { return yesNo(r.ProceedingsPending) },
	},
	{
		Key: "remarks", Group: GroupLocal,
		Label: "Remarks", Column: "Remarks",
		Help:      "Anything the form asks to be specified that has no box of its own here. Checked like the other free-text fields.",
		Kind:      KindLongText,
		Statutory: false,
		Screened:  true,
		filled:    func(r Record) bool { return trimmed(r.Remarks) },
		render:    func(r Record) string { return r.Remarks },
	},
}

func yesNo(b bool) string {
	if b {
		return "Yes"
	}
	return "No"
}

func intOrBlank(n int) string {
	if n <= 0 {
		return ""
	}
	return strconv.Itoa(n)
}

func hasChoice(list []string, want string) bool {
	for _, s := range list {
		if s == want {
			return true
		}
	}
	return false
}

// joinIndications renders the chosen indications with the numbers the form
// gives them, so that a printed page can be checked against the paper tick.
func joinIndications(chosen []string) string {
	parts := make([]string, 0, len(chosen))
	for _, c := range chosen {
		if n := IndicationNumber(c); n != "" && n != "—" {
			parts = append(parts, n+" "+c)
			continue
		}
		parts = append(parts, c)
	}
	return strings.Join(parts, "; ")
}
