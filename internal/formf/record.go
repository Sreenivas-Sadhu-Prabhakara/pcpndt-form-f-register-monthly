package formf

import "strings"

// What Form F is, and which part of it this register keeps.
//
// Form F is prescribed by rule 9(4) of the Pre-conception and Pre-natal
// Diagnostic Techniques (Prohibition of Sex Selection) Rules, 1996. The form
// in force was substituted by G.S.R. 77(E) dated 31 January 2014 with effect
// from 4 February 2014, and it prints 27 numbered items in four sections:
//
//	Section A  items 1-8    for all diagnostic procedures and tests
//	Section B  items 9-16   for non-invasive procedures only
//	Section C  items 17-27  for invasive procedures only
//	Section D               the two declarations
//
// This register keeps Sections A, B and D — the sections an ultrasound clinic
// or imaging centre fills in. It does not reproduce Section C, which belongs
// to the invasive pre-natal diagnostic procedures carried out at a genetic
// clinic: amniocentesis, chorionic villi aspiration, cordocentesis and the
// like. A centre registered for ultrasound alone is not registered to perform
// any of them, and a register that offered the fields would invite an entry
// nobody at the centre is entitled to make. Where a centre does hold that
// registration, Section C has to be completed on the form itself and this
// register will say so rather than pretend to cover it.
//
// Two further things are worth knowing before reading the fields.
//
// The serial number is not an item of Form F. The form has no serial number
// field at all — it opens at item 1 with the centre's name. What is prescribed
// is separate: rule 9(1) requires the centre to keep a register showing, in
// serial order, the name and address of each woman, the name of her spouse or
// father, and the date she first reported. That register is the one printed
// for inspection by this tool, with exactly those columns; the serial number
// here is the number in it.
//
// One item of Section A is deliberately not stored in full. See ItemsNotStored.

// Record is one Form F entry: one procedure, on one woman, on one day.
type Record struct {
	// Serial is this entry's number in the rule 9(1) register. It is
	// assigned by the register and never typed in; see serial.go.
	Serial int `json:"serial"`

	// FirstReportedOn is the date she first reported for the procedure,
	// which is a prescribed column of the rule 9(1) register rather than
	// an item of Form F.
	FirstReportedOn string `json:"firstReportedOn"`

	// ---- Section A: for all diagnostic procedures and tests ----

	// PatientName and Age are item 3.
	PatientName string `json:"patientName"`
	Age         int    `json:"age"`

	// LivingChildren is the total asked for by item 4. It is a pointer
	// because none is a real and common answer that has to be
	// distinguishable from an unanswered item: a zero means she has no
	// living children, a nil means nobody has filled the item in.
	LivingChildren *int `json:"livingChildren"`
	// ChildAges holds the ages item 4 asks for, as written.
	ChildAges string `json:"childAges"`

	// RelativeName is item 5, which asks for the husband's, wife's,
	// father's or mother's name.
	RelativeName string `json:"relativeName"`

	// Address and Phone are item 6.
	Address string `json:"address"`
	Phone   string `json:"phone"`

	// Referral is item 7. Either 7(a) names the referring doctor or
	// centre, or 7(b) records a self-referral — and self-referral has a
	// narrow meaning the form spells out on its face.
	SelfReferral        bool   `json:"selfReferral"`
	ReferrerName        string `json:"referrerName"`
	ReferrerAddress     string `json:"referrerAddress"`
	ReferrerQualificatn string `json:"referrerQualification"`
	ReferralPapersHeld  bool   `json:"referralPapersHeld"`

	// LMP and GestationWeeks are item 8.
	LMP            string `json:"lmp"`
	GestationWeeks int    `json:"gestationWeeks"`

	// ---- Section B: non-invasive procedures only ----

	// ConductedByName is item 9.
	ConductedByName string `json:"conductedByName"`

	// Indications is item 10, chosen from the list the form prints, and
	// IndicationDetail is the clinician's own wording, which the form asks
	// to be given with reference to the referral slip or self-referral
	// note.
	Indications      []string `json:"indications"`
	IndicationDetail string   `json:"indicationDetail"`

	// Procedures is item 11 and ProcedureOther is its "any other".
	Procedures     []string `json:"procedures"`
	ProcedureOther string   `json:"procedureOther"`

	// WomanDeclarationDate is item 12.
	WomanDeclarationDate string `json:"womanDeclarationDate"`

	// ProcedureDate is item 13. It places the entry in a month's return
	// and starts the retention clock.
	ProcedureDate string `json:"procedureDate"`

	// Result is item 14: the report in brief of the procedure carried out.
	Result string `json:"result"`

	// ConveyedTo and ConveyedOn are item 15.
	ConveyedTo string `json:"conveyedTo"`
	ConveyedOn string `json:"conveyedOn"`

	// MTPIndication and MTPDetail are item 16, which asks whether anything
	// detected gives an indication for a medical termination.
	MTPIndication string `json:"mtpIndication"`
	MTPDetail     string `json:"mtpDetail"`

	// The Section B signature block: name, signature and registration
	// number with seal, of the person who performed the procedure, with
	// the date and place.
	SignatoryName  string `json:"signatoryName"`
	SignatoryRegNo string `json:"signatoryRegNo"`
	SealAffixed    bool   `json:"sealAffixed"`
	SignedOn       string `json:"signedOn"`
	Place          string `json:"place"`

	// ---- Section D: the declarations ----

	// DeclarationLanguage records the language her declaration was given
	// in, which is what an inspection asks about: a declaration in a
	// language she does not read is not a declaration.
	DeclarationLanguage string `json:"declarationLanguage"`

	// Where she signs with a thumb impression the form requires the
	// identifying person's particulars and an attesting signature.
	ThumbImpression    bool   `json:"thumbImpression"`
	IdentifierName     string `json:"identifierName"`
	IdentifierAge      int    `json:"identifierAge"`
	IdentifierRelation string `json:"identifierRelation"`
	IdentifierAddress  string `json:"identifierAddress"`
	AttestedBy         string `json:"attestedBy"`
	AttestedOn         string `json:"attestedOn"`

	// The second declaration, by the person who conducted the procedure.
	DoctorDeclarationSigned bool   `json:"doctorDeclarationSigned"`
	DoctorDeclarationDate   string `json:"doctorDeclarationDate"`

	// ---- Kept by this register, not asked for by the form ----

	// Equipment identifies the machine, so that a month can be reconciled
	// against one machine's own log.
	Equipment string `json:"equipment"`
	// ProceedingsPending is noted against the record because the
	// retention proviso turns on it.
	ProceedingsPending bool   `json:"proceedingsPending"`
	Remarks            string `json:"remarks"`
}

// Clinic is the registered facility keeping the register. It sits once in the
// data file rather than being copied onto every entry, and it supplies items 1
// and 2 of every Form F and the address block of every return.
type Clinic struct {
	// Name and Address are item 1.
	Name    string `json:"name"`
	Address string `json:"address"`
	// RegistrationNo is item 2: the registration under the Act, not a
	// shop licence or a clinical establishment number.
	RegistrationNo string `json:"registrationNo"`
	District       string `json:"district"`
	State          string `json:"state"`
	// AppropriateAuthority is the office the monthly return is addressed
	// to, which is the district Appropriate Authority for most centres.
	AppropriateAuthority string `json:"appropriateAuthority"`
}

// Item 16 answers. The form asks whether anything detected gives an
// indication for a medical termination of pregnancy.
const (
	MTPNone      = "None"
	MTPIndicated = "Yes — stated below"
)

// MTPAnswers returns the selectable answers to item 16.
func MTPAnswers() []string { return []string{MTPNone, MTPIndicated} }

// Item 11 procedures. Section B of the form lists only the two: ultrasound,
// and anything else to be specified. The invasive techniques are item 21, in
// Section C, which this register does not keep.
const (
	ProcUltrasound = "Ultrasound"
	ProcOther      = "Any other — state it below"
)

// Procedures returns the selectable procedures for item 11.
func Procedures() []string { return []string{ProcUltrasound, ProcOther} }

// invasiveTechniques are the Section C techniques, matched against whatever is
// typed into item 11's "any other" box. An ultrasound clinic writing one of
// these into Section B has either used a technique its registration does not
// cover or is filling in the wrong section, and both are worth stopping on.
var invasiveTechniques = []string{
	"amniocentesis", "chorionic", "villi", "villus",
	"cordocentesis", "cvs", "foetal biopsy", "fetal biopsy", "foetal blood sampling",
	"fetal blood sampling", "skin biopsy",
}

// MentionsInvasiveTechnique returns the invasive techniques named in a string,
// lowercased and deduplicated.
func MentionsInvasiveTechnique(s string) []string {
	low := strings.ToLower(s)
	seen := map[string]bool{}
	var out []string
	for _, t := range invasiveTechniques {
		if seen[t] || !strings.Contains(low, t) {
			continue
		}
		seen[t] = true
		out = append(out, t)
	}
	return out
}

// trimmed reports whether a string holds anything once surrounding space is
// taken off. A field containing a single space is blank on paper too.
func trimmed(s string) bool { return strings.TrimSpace(s) != "" }

// nonEmpty reports whether a list holds at least one non-blank entry.
func nonEmpty(list []string) bool {
	for _, s := range list {
		if trimmed(s) {
			return true
		}
	}
	return false
}
