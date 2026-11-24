package formf

import "testing"

// indicationNum returns the indication the form numbers with num, so that
// tests name an indication by its number on the paper form rather than by a
// position in a slice.
func indicationNum(num string) string {
	for _, i := range Indications() {
		if i.Num == num {
			return i.Text
		}
	}
	return ""
}

func intPtr(n int) *int { return &n }

// completeRecord is an entry with every applicable item filled in and nothing
// contradicting anything else. Tests start from it and break one thing at a
// time, which is the only way to be sure a check fires for the reason it is
// supposed to.
//
// The dates hold together on purpose: she first reported on the 16th, gave her
// declaration and was scanned on the 18th, and a last menstrual period of
// 30 June 2026 puts her at 20 completed weeks on 18 November 2026.
func completeRecord() Record {
	return Record{
		Serial:          1,
		FirstReportedOn: "2026-11-16",

		PatientName:    "Shalini Kamath",
		Age:            29,
		LivingChildren: intPtr(1),
		ChildAges:      "4 y",
		RelativeName:   "Rohit Kamath",
		Address:        "12 Gokhale Path, Dharwad 580001, Karnataka",
		Phone:          "9876500000",

		SelfReferral:        false,
		ReferrerName:        "Dr A. Kulkarni",
		ReferrerAddress:     "Kulkarni Nursing Home, Vidyanagar, Dharwad 580021",
		ReferrerQualificatn: "MBBS, MS (Obst. & Gyn.)",
		ReferralPapersHeld:  true,

		LMP:            "2026-06-30",
		GestationWeeks: 20,

		ConductedByName:  "Dr M. Rao",
		Indications:      []string{indicationNum("(xvii)")},
		IndicationDetail: "Referral slip asks for a scan of growth parameters at 20 weeks.",
		Procedures:       []string{ProcUltrasound},

		WomanDeclarationDate: "2026-11-18",
		ProcedureDate:        "2026-11-18",
		Result:               "Single live intrauterine pregnancy. Biometry corresponds to 20 weeks 1 day. No structural abnormality seen. Liquor adequate, placenta posterior and upper segment.",
		ConveyedTo:           "The patient, in person at the centre",
		ConveyedOn:           "2026-11-18",
		MTPIndication:        MTPNone,

		SignatoryName:  "Dr M. Rao",
		SignatoryRegNo: "KMC/33104",
		SealAffixed:    true,
		SignedOn:       "2026-11-18",
		Place:          "Dharwad",

		DeclarationLanguage:     "Kannada",
		DoctorDeclarationSigned: true,
		DoctorDeclarationDate:   "2026-11-18",

		Equipment: "USG-2, bay 1",
	}
}

// TestCompleteRecordIsComplete guards every other test in this package: if the
// fixture stops being a complete entry, a test that breaks one field and
// expects one issue would silently start passing for the wrong reason.
func TestCompleteRecordIsComplete(t *testing.T) {
	v := Validate(completeRecord())
	if !v.Complete() {
		for _, iss := range v.Issues {
			t.Errorf("[%s] %s: %s", iss.Kind, iss.Field, iss.Message)
		}
		t.Fatalf("the fixture is not a complete entry: %s", v.Summary)
	}
	if v.Mandatory != v.Filled {
		t.Errorf("Mandatory = %d but Filled = %d on a complete entry", v.Mandatory, v.Filled)
	}
	if v.Mandatory < 20 {
		t.Errorf("only %d mandatory items were checked; the prescribed set is larger than that", v.Mandatory)
	}
}

func testClinic() Clinic {
	return Clinic{
		Name:                 "Sahyadri Imaging Centre",
		Address:              "3rd Cross, Court Circle, Dharwad 580001, Karnataka",
		RegistrationNo:       "PCPNDT/DHW/2019/0147",
		District:             "Dharwad",
		State:                "Karnataka",
		AppropriateAuthority: "District Appropriate Authority, Dharwad",
	}
}

func testSettings() Settings {
	s := DefaultSettings()
	s.ResponsiblePerson = "Dr M. Rao"
	return s
}
