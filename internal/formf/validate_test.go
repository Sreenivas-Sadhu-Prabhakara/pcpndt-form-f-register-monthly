package formf

import (
	"reflect"
	"strings"
	"testing"
	"time"
)

// keysOf returns the fields named by a set of issues, deduplicated and in the
// order they were reported.
func keysOf(issues []Issue) []string {
	seen := map[string]bool{}
	var out []string
	for _, i := range issues {
		if seen[i.Field] {
			continue
		}
		seen[i.Field] = true
		out = append(out, i.Field)
	}
	return out
}

// TestValidateNamesEveryMissingMandatoryFieldAtOnce is the central claim of
// this package. An empty entry has to come back with the whole shortfall, in
// the order the paperwork prints the items — not the first blank, not a count.
func TestValidateNamesEveryMissingMandatoryFieldAtOnce(t *testing.T) {
	v := Validate(Record{})

	want := []string{
		// rule 9(1) register
		"serial", "firstReportedOn",
		// Section A
		"patientName", "age", "livingChildren", "relativeName", "address",
		"referrerName", "referrerAddress", "referrerQualification", "referralPapersHeld",
		"lmp", "gestationWeeks",
		// Section B
		"conductedByName", "indications", "indicationDetail", "procedures",
		"womanDeclarationDate", "procedureDate", "result", "mtpIndication",
		// Section B signature block
		"signatoryName", "signatoryRegNo", "sealAffixed", "signedOn", "place",
		// Section D
		"declarationLanguage", "doctorDeclarationSigned", "doctorDeclarationDate",
	}
	got := v.MissingFields()
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Validate(Record{}).MissingFields() =\n  %v\nwant\n  %v", got, want)
	}
	if len(v.Missing()) != len(want) {
		t.Errorf("Missing() returned %d issues for %d blank items", len(v.Missing()), len(want))
	}
	if v.Mandatory != len(want) {
		t.Errorf("Mandatory = %d, want %d: the count of applicable items and the count of blanks must agree on an empty entry", v.Mandatory, len(want))
	}
	if v.Filled != 0 {
		t.Errorf("Filled = %d on an empty entry", v.Filled)
	}
	if v.Complete() {
		t.Error("an empty entry reported itself complete")
	}
	if !v.Storable() {
		t.Error("an empty entry is not storable; a blank has to be correctable later, unlike refused text")
	}

	// Every message has to be actionable on its own: it names the item
	// number where the form has one, and says what the item wants.
	for _, iss := range v.Missing() {
		if iss.Label == "" {
			t.Errorf("missing-field issue for %q carries no label", iss.Field)
		}
		if iss.Item == "" {
			t.Errorf("missing-field issue for %q carries no item number", iss.Field)
		}
		if !strings.Contains(iss.Message, "blank") {
			t.Errorf("message for %q does not say the item is blank: %q", iss.Field, iss.Message)
		}
		if len(iss.Message) < 40 {
			t.Errorf("message for %q is too terse to act on: %q", iss.Field, iss.Message)
		}
	}
}

// TestValidateOneThingAtATime breaks a single field on an otherwise complete
// entry and checks that exactly the expected checks fire.
func TestValidateOneThingAtATime(t *testing.T) {
	cases := []struct {
		name            string
		mutate          func(*Record)
		wantMissing     []string
		wantInconsistnt []string
	}{
		{name: "nothing broken"},

		{
			name:        "her name left blank",
			mutate:      func(r *Record) { r.PatientName = "  " },
			wantMissing: []string{"patientName"},
		},
		{
			name:        "age left blank",
			mutate:      func(r *Record) { r.Age = 0 },
			wantMissing: []string{"age"},
		},
		{
			name:        "the living-children item unanswered",
			mutate:      func(r *Record) { r.LivingChildren = nil },
			wantMissing: []string{"livingChildren"},
		},
		{
			name:   "no living children, and no ages",
			mutate: func(r *Record) { r.LivingChildren = intPtr(0); r.ChildAges = "" },
		},
		{
			name:            "no living children but ages entered anyway",
			mutate:          func(r *Record) { r.LivingChildren = intPtr(0) },
			wantInconsistnt: []string{"childAges"},
		},
		{
			name:        "children but no ages",
			mutate:      func(r *Record) { r.LivingChildren = intPtr(3); r.ChildAges = "" },
			wantMissing: []string{"childAges"},
		},
		{
			name:            "an implausible number of children",
			mutate:          func(r *Record) { r.LivingChildren = intPtr(44) },
			wantInconsistnt: []string{"livingChildren"},
		},
		{
			name:            "an implausible age",
			mutate:          func(r *Record) { r.Age = 150 },
			wantInconsistnt: []string{"age"},
		},

		{
			name: "no referral and no self-referral",
			mutate: func(r *Record) {
				r.ReferrerName, r.ReferrerAddress, r.ReferrerQualificatn = "", "", ""
			},
			wantMissing: []string{"referrerName", "referrerAddress", "referrerQualification"},
		},
		{
			name: "a genuine self-referral",
			mutate: func(r *Record) {
				r.SelfReferral = true
				r.ReferrerName, r.ReferrerAddress, r.ReferrerQualificatn = "", "", ""
			},
		},
		{
			name:            "marked a self-referral and also referred by someone",
			mutate:          func(r *Record) { r.SelfReferral = true },
			wantInconsistnt: []string{"selfReferral"},
		},
		{
			name:        "the referral slip is not in the file",
			mutate:      func(r *Record) { r.ReferralPapersHeld = false },
			wantMissing: []string{"referralPapersHeld"},
		},

		{
			name:            "she first reported after the scan",
			mutate:          func(r *Record) { r.FirstReportedOn = "2026-11-20" },
			wantInconsistnt: []string{"firstReportedOn"},
		},
		{
			name:            "she first reported a year earlier",
			mutate:          func(r *Record) { r.FirstReportedOn = "2025-11-16" },
			wantInconsistnt: []string{"firstReportedOn"},
		},
		{
			name:            "her declaration taken after the procedure",
			mutate:          func(r *Record) { r.WomanDeclarationDate = "2026-11-19" },
			wantInconsistnt: []string{"womanDeclarationDate"},
		},
		{
			name:            "her declaration taken two months early",
			mutate:          func(r *Record) { r.WomanDeclarationDate = "2026-09-01" },
			wantInconsistnt: []string{"womanDeclarationDate"},
		},
		{
			name:            "the entry signed before the procedure",
			mutate:          func(r *Record) { r.SignedOn = "2026-11-17" },
			wantInconsistnt: []string{"signedOn"},
		},
		{
			name:            "the entry signed three weeks later",
			mutate:          func(r *Record) { r.SignedOn = "2026-12-09" },
			wantInconsistnt: []string{"signedOn"},
		},
		{
			name:            "the conductor's declaration dated before the procedure",
			mutate:          func(r *Record) { r.DoctorDeclarationDate = "2026-11-17" },
			wantInconsistnt: []string{"doctorDeclarationDate"},
		},
		{
			name:        "the conductor's declaration unsigned",
			mutate:      func(r *Record) { r.DoctorDeclarationSigned = false },
			wantMissing: []string{"doctorDeclarationSigned"},
		},
		{
			name:        "no seal on the signed form",
			mutate:      func(r *Record) { r.SealAffixed = false },
			wantMissing: []string{"sealAffixed"},
		},
		{
			name:        "no language recorded for her declaration",
			mutate:      func(r *Record) { r.DeclarationLanguage = "" },
			wantMissing: []string{"declarationLanguage"},
		},

		{
			name:            "the weeks do not follow from the last menstrual period",
			mutate:          func(r *Record) { r.GestationWeeks = 12 },
			wantInconsistnt: []string{"gestationWeeks"},
		},
		{
			name:   "the weeks are two off, which is within tolerance",
			mutate: func(r *Record) { r.GestationWeeks = 22 },
		},
		{
			name:            "the last menstrual period falls after the procedure",
			mutate:          func(r *Record) { r.LMP = "2026-11-19" },
			wantInconsistnt: []string{"lmp"},
		},
		{
			name:            "the last menstrual period is a year out",
			mutate:          func(r *Record) { r.LMP = "2025-06-30" },
			wantInconsistnt: []string{"lmp"},
		},
		{
			name:            "an impossible gestational age",
			mutate:          func(r *Record) { r.GestationWeeks = 60 },
			wantInconsistnt: []string{"gestationWeeks"},
		},

		{
			name:        "another procedure, unspecified",
			mutate:      func(r *Record) { r.Procedures = []string{ProcOther} },
			wantMissing: []string{"procedureOther"},
		},
		{
			name: "an invasive technique written into Section B",
			mutate: func(r *Record) {
				r.Procedures = []string{ProcOther}
				r.ProcedureOther = "Amniocentesis under ultrasound guidance"
			},
			wantInconsistnt: []string{"procedureOther"},
		},
		{
			name:        "no procedure recorded at all",
			mutate:      func(r *Record) { r.Procedures = nil },
			wantMissing: []string{"procedures"},
		},

		{
			name:        "no indication recorded at all",
			mutate:      func(r *Record) { r.Indications = nil },
			wantMissing: []string{"indications"},
		},
		{
			name:        "the indication not put into words against the referral",
			mutate:      func(r *Record) { r.IndicationDetail = "" },
			wantMissing: []string{"indicationDetail"},
		},
		{
			name:            "an indication that is not on the form's list",
			mutate:          func(r *Record) { r.Indications = []string{"The family asked for a scan"} },
			wantInconsistnt: []string{"indications"},
		},
		{
			name:   "the register's own entry for an indication outside the list",
			mutate: func(r *Record) { r.Indications = []string{IndOther} },
		},

		{
			name:        "a termination indicated with nothing stated",
			mutate:      func(r *Record) { r.MTPIndication = MTPIndicated },
			wantMissing: []string{"mtpDetail"},
		},
		{
			name:            "no termination indicated but details entered",
			mutate:          func(r *Record) { r.MTPDetail = "Advised review in two weeks" },
			wantInconsistnt: []string{"mtpDetail"},
		},
		{
			name:        "item 16 unanswered",
			mutate:      func(r *Record) { r.MTPIndication = "" },
			wantMissing: []string{"mtpIndication"},
		},

		{
			name:            "the result conveyed to nobody in particular",
			mutate:          func(r *Record) { r.ConveyedTo = "" },
			wantInconsistnt: []string{"conveyedTo"},
		},
		{
			name:            "conveyed to someone on no date",
			mutate:          func(r *Record) { r.ConveyedOn = "" },
			wantInconsistnt: []string{"conveyedOn"},
		},
		{
			name:            "conveyed before the procedure happened",
			mutate:          func(r *Record) { r.ConveyedOn = "2026-11-10" },
			wantInconsistnt: []string{"conveyedOn"},
		},
		{
			name:   "item 15 left entirely blank, which it may be",
			mutate: func(r *Record) { r.ConveyedTo, r.ConveyedOn = "", "" },
		},

		{
			name:        "a thumb impression with none of the particulars",
			mutate:      func(r *Record) { r.ThumbImpression = true },
			wantMissing: []string{"identifierName", "identifierAge", "identifierAddress", "attestedBy", "attestedOn"},
		},
		{
			name: "a thumb impression, properly identified and attested",
			mutate: func(r *Record) {
				r.ThumbImpression = true
				r.IdentifierName = "Savitri Bai"
				r.IdentifierAge = 52
				r.IdentifierRelation = "Mother-in-law"
				r.IdentifierAddress = "12 Gokhale Path, Dharwad 580001 — 9876500001"
				r.AttestedBy = "Dr M. Rao"
				r.AttestedOn = "2026-11-18"
			},
		},
		{
			name: "attested before the declaration it attests",
			mutate: func(r *Record) {
				r.ThumbImpression = true
				r.IdentifierName = "Savitri Bai"
				r.IdentifierAge = 52
				r.IdentifierAddress = "12 Gokhale Path, Dharwad 580001"
				r.AttestedBy = "Dr M. Rao"
				r.AttestedOn = "2026-11-15"
			},
			wantInconsistnt: []string{"attestedOn"},
		},
		{
			name:            "identifying particulars with no thumb impression",
			mutate:          func(r *Record) { r.IdentifierName = "Savitri Bai" },
			wantInconsistnt: []string{"thumbImpression"},
		},

		{
			name:            "signed by somebody other than the person who performed it",
			mutate:          func(r *Record) { r.SignatoryName = "Dr S. Patil" },
			wantInconsistnt: []string{"signatoryName"},
		},
		{
			name:   "the same person, written differently",
			mutate: func(r *Record) { r.SignatoryName = "Dr. M Rao" },
		},
		{
			name:        "no registration number beside the signature",
			mutate:      func(r *Record) { r.SignatoryRegNo = "" },
			wantMissing: []string{"signatoryRegNo"},
		},

		{
			name:            "a procedure date in the wrong format",
			mutate:          func(r *Record) { r.ProcedureDate = "18-11-2026" },
			wantMissing:     []string{"procedureDate"},
			wantInconsistnt: []string{"procedureDate"},
		},
		{
			name:            "a date that does not exist",
			mutate:          func(r *Record) { r.FirstReportedOn = "2026-02-30" },
			wantMissing:     []string{"firstReportedOn"},
			wantInconsistnt: []string{"firstReportedOn"},
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			r := completeRecord()
			if c.mutate != nil {
				c.mutate(&r)
			}
			v := Validate(r)

			if got := keysOf(v.Missing()); !reflect.DeepEqual(got, c.wantMissing) {
				t.Errorf("missing fields = %v, want %v", got, c.wantMissing)
			}
			if got := keysOf(v.Inconsistent()); !reflect.DeepEqual(got, c.wantInconsistnt) {
				t.Errorf("inconsistent fields = %v, want %v", got, c.wantInconsistnt)
				for _, iss := range v.Inconsistent() {
					t.Logf("  %s: %s", iss.Field, iss.Message)
				}
			}
			if n := len(v.Refused()); n != 0 {
				t.Errorf("%d field(s) refused, want none in this test", n)
			}
			wantComplete := len(c.wantMissing) == 0 && len(c.wantInconsistnt) == 0
			if v.Complete() != wantComplete {
				t.Errorf("Complete() = %v, want %v (%s)", v.Complete(), wantComplete, v.Summary)
			}
			// Whatever is wrong, a storable entry stays storable: only
			// refused text stops an entry being written at all.
			if !v.Storable() {
				t.Error("Storable() = false with nothing refused")
			}
		})
	}
}

func TestValidateOnFlagsFutureDates(t *testing.T) {
	today := Day(2026, time.November, 24)
	cases := []struct {
		name  string
		mutat func(*Record)
		want  []string
	}{
		{"nothing in the future", func(*Record) {}, nil},
		{"the procedure itself", func(r *Record) {
			r.ProcedureDate, r.SignedOn, r.DoctorDeclarationDate, r.WomanDeclarationDate, r.ConveyedOn = "2026-12-02", "2026-12-02", "2026-12-02", "2026-12-02", "2026-12-02"
		}, []string{"womanDeclarationDate", "procedureDate", "conveyedOn", "signedOn", "doctorDeclarationDate"}},
		{"only the signature", func(r *Record) { r.SignedOn = "2026-11-25" }, []string{"signedOn"}},
		{"today is not the future", func(r *Record) {
			r.ProcedureDate, r.SignedOn, r.DoctorDeclarationDate, r.WomanDeclarationDate, r.ConveyedOn = "2026-11-24", "2026-11-24", "2026-11-24", "2026-11-24", "2026-11-24"
		}, nil},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			r := completeRecord()
			c.mutat(&r)

			// Validate without a reference day must not raise any of
			// these, which is what keeps it a pure function.
			plain := Validate(r)
			for _, iss := range plain.Inconsistent() {
				if strings.Contains(iss.Message, "in the future") {
					t.Errorf("Validate raised a future-date issue without a reference day: %s", iss.Message)
				}
			}

			var got []string
			for _, iss := range ValidateOn(r, today).Inconsistent() {
				if strings.Contains(iss.Message, "in the future") {
					got = append(got, iss.Field)
				}
			}
			if !reflect.DeepEqual(got, c.want) {
				t.Errorf("future-dated fields = %v, want %v", got, c.want)
			}
		})
	}
}

func TestValidationSummary(t *testing.T) {
	cases := []struct {
		name   string
		mutate func(*Record)
		want   string
	}{
		// Thirty, not twenty-nine: the fixture records a living child,
		// which makes the ages item apply too.
		{"complete", func(*Record) {}, "Complete: all 30 applicable items are filled in."},
		{"one blank", func(r *Record) { r.Place = "" }, "Incomplete: 1 mandatory item blank."},
		{"two blanks", func(r *Record) { r.Place, r.SignatoryRegNo = "", "" }, "Incomplete: 2 mandatory items blank."},
		{"one contradiction", func(r *Record) { r.SignedOn = "2026-11-01" }, "Incomplete: 1 answer that cannot be right together."},
		{"a blank and a refusal", func(r *Record) { r.Place = ""; r.Remarks = "wants a boy" },
			"Incomplete: 1 mandatory item blank, 1 field refused."},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			r := completeRecord()
			c.mutate(&r)
			if got := Validate(r).Summary; got != c.want {
				t.Errorf("Summary = %q, want %q", got, c.want)
			}
		})
	}
}

func TestEveryCatalogueFieldHasWhatAMessageNeeds(t *testing.T) {
	seen := map[string]bool{}
	for _, f := range Fields() {
		if f.Key == "" {
			t.Error("a field has no key")
		}
		if seen[f.Key] {
			t.Errorf("duplicate field key %q", f.Key)
		}
		seen[f.Key] = true
		if f.Label == "" {
			t.Errorf("%s has no label", f.Key)
		}
		if f.Column == "" {
			t.Errorf("%s has no column heading", f.Key)
		}
		if f.Group == "" {
			t.Errorf("%s has no group", f.Key)
		}
		if f.Help == "" {
			t.Errorf("%s has no help text, so its blank message would say nothing useful", f.Key)
		}
		if f.Kind == "" {
			t.Errorf("%s has no kind, so no interface can render it", f.Key)
		}
		if f.Conditional && f.When == "" {
			t.Errorf("%s is conditional but does not say when it applies", f.Key)
		}
		if f.Statutory && f.Item == "" {
			t.Errorf("%s is marked statutory but carries no item or rule number", f.Key)
		}
		if !f.Statutory && f.Item != "" {
			t.Errorf("%s is not statutory but carries item number %q", f.Key, f.Item)
		}
		if (f.Kind == KindChoice || f.Kind == KindMulti) && len(f.Choices) == 0 {
			t.Errorf("%s is a choice with no choices", f.Key)
		}
		if _, ok := FieldByKey(f.Key); !ok {
			t.Errorf("FieldByKey cannot find %q", f.Key)
		}
	}
	if _, ok := FieldByKey("noSuchField"); ok {
		t.Error("FieldByKey found a field that does not exist")
	}
	if len(Fields()) < 35 {
		t.Errorf("the catalogue holds only %d fields; the sections this register keeps are longer than that", len(Fields()))
	}
}

func TestFieldsIsACopy(t *testing.T) {
	first := Fields()
	first[0].Label = "mutated"
	if Fields()[0].Label == "mutated" {
		t.Error("Fields() hands out the live catalogue; a caller could rewrite the prescribed field set")
	}
}

func TestMentionsInvasiveTechnique(t *testing.T) {
	cases := []struct {
		in   string
		want []string
	}{
		{"", nil},
		{"Transabdominal ultrasound", nil},
		{"Colour Doppler study", nil},
		{"Amniocentesis", []string{"amniocentesis"}},
		{"amniocentesis under guidance", []string{"amniocentesis"}},
		{"Chorionic villus sampling", []string{"chorionic", "villus"}},
		{"CVS at 12 weeks", []string{"cvs"}},
		{"Cordocentesis", []string{"cordocentesis"}},
		{"Foetal biopsy", []string{"foetal biopsy"}},
		{"Foetal skin biopsy", []string{"skin biopsy"}},
	}
	for _, c := range cases {
		t.Run(c.in, func(t *testing.T) {
			if got := MentionsInvasiveTechnique(c.in); !reflect.DeepEqual(got, c.want) {
				t.Errorf("MentionsInvasiveTechnique(%q) = %v, want %v", c.in, got, c.want)
			}
		})
	}
}
