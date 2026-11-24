package formf

// Item 10: the indications.
//
// Item 10 of Form F asks for the indication for the procedure, "specify with
// reference to the request made in the referral slip or in a self-referral
// note", and prints a list to tick against. The form introduces the list as a
// representative one, and prefaces it with the statement that ultrasonography
// in pregnancy should only be performed when indicated.
//
// The twenty-three indications below are that list, in the form's own order
// and roman numbering. An entry with no indication recorded at all is the
// first thing an inspection looks for, because a procedure with no reason
// given is a procedure nobody can account for — and under the proviso to
// section 4(3) a deficiency in the record is itself treated as a contravention
// unless the person conducting the procedure proves otherwise.
//
// Wording: reproductions of the form differ between themselves on spelling —
// "fetal" against "foetal" within the same printed list, and at least one
// spelling the form itself carries that a dictionary would not. The list here
// is spelled consistently with "foetal" and otherwise follows the form,
// including its own "adenexal". Nothing of substance has been reworded. Where
// the form in circulation in your state differs, record the indication as it
// is worded there in the box beside the list and treat the form in your
// inspection file as the authority.

// Indication is one entry of item 10's list, with the number the form gives it
// so that a tick here can be matched to a tick on the paper form.
type Indication struct {
	Num  string `json:"num"`
	Text string `json:"text"`
	// Listed is false for the one entry this register adds, which exists
	// because the form calls its own list representative.
	Listed bool `json:"listed"`
}

// IndOther is the entry this register adds to item 10's list. The form
// describes the twenty-three as representative, and item 10 asks for the
// indication to be specified in any event, so an indication outside the list
// is recorded in words rather than squeezed into the nearest tick.
const IndOther = "An indication not in the printed list — state it in the box below"

var indications = []Indication{
	{"(i)", "To diagnose intra-uterine and/or ectopic pregnancy and confirm viability", true},
	{"(ii)", "Estimation of gestational age (dating)", true},
	{"(iii)", "Detection of number of foetuses and their chorionicity", true},
	{"(iv)", "Suspected pregnancy with IUCD in-situ or suspected pregnancy following contraceptive failure/MTP failure", true},
	{"(v)", "Vaginal bleeding/leaking", true},
	{"(vi)", "Follow-up of cases of abortion", true},
	{"(vii)", "Assessment of cervical canal and diameter of internal os", true},
	{"(viii)", "Discrepancy between uterine size and period of amenorrhea", true},
	{"(ix)", "Any suspected adenexal or uterine pathology/abnormality", true},
	{"(x)", "Detection of chromosomal abnormalities, foetal structural defects and other abnormalities and their follow-up", true},
	{"(xi)", "To evaluate foetal presentation and position", true},
	{"(xii)", "Assessment of liquor amnii", true},
	{"(xiii)", "Preterm labour/preterm premature rupture of membranes", true},
	{"(xiv)", "Evaluation of placental position, thickness, grading and abnormalities (placenta praevia, retro placental haemorrhage, abnormal adherence etc.)", true},
	{"(xv)", "Evaluation of umbilical cord — presentation, insertion, nuchal encirclement, number of vessels and presence of true knot", true},
	{"(xvi)", "Evaluation of previous Caesarean Section scars", true},
	{"(xvii)", "Evaluation of foetal growth parameters, foetal weight and foetal well being", true},
	{"(xviii)", "Colour flow mapping and duplex Doppler studies", true},
	{"(xix)", "Ultrasound guided procedures such as medical termination of pregnancy, external cephalic version etc. and their follow-up", true},
	{"(xx)", "Adjunct to diagnostic and therapeutic invasive interventions such as chorionic villus sampling (CVS), amniocenteses, foetal blood sampling, foetal skin biopsy, amnio-infusion, intrauterine infusion, placement of shunts etc.", true},
	{"(xxi)", "Observation of intra-partum events", true},
	{"(xxii)", "Medical/surgical conditions complicating pregnancy", true},
	{"(xxiii)", "Research/scientific studies in recognised institutions", true},
	{"—", IndOther, false},
}

// Indications returns item 10's list, in the form's order.
func Indications() []Indication {
	out := make([]Indication, len(indications))
	copy(out, indications)
	return out
}

// IndicationTexts returns just the wording of each indication, which is what
// a record stores and what a form prints.
func IndicationTexts() []string {
	out := make([]string, 0, len(indications))
	for _, i := range indications {
		out = append(out, i.Text)
	}
	return out
}

// IsListedIndication reports whether a string is one of the indications the
// form prints, so that a clinician's own wording can be told apart from a tick
// against the list.
func IsListedIndication(s string) bool {
	for _, i := range indications {
		if i.Text == s && i.Listed {
			return true
		}
	}
	return false
}

// IndicationNumber returns the number the form gives an indication, or an
// empty string for wording that is not on the list.
func IndicationNumber(s string) string {
	for _, i := range indications {
		if i.Text == s {
			return i.Num
		}
	}
	return ""
}

// CountListedIndications returns how many of the chosen indications are on the
// printed list.
func CountListedIndications(chosen []string) int {
	n := 0
	for _, c := range chosen {
		if IsListedIndication(c) {
			n++
		}
	}
	return n
}
