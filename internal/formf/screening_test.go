package formf

import (
	"reflect"
	"testing"
)

func TestScreenText(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want []string
	}{
		// Clinical language the guard must leave alone. If any of these were
		// refused the clinic would switch the guard off, and then it guards
		// nothing at all.
		{"fetal heart rate", "Fetal heart rate 148 bpm, regular.", nil},
		{"foetal biometry", "Foetal biometry corresponds to 19 weeks 2 days.", nil},
		{"beta hCG", "Beta hCG 1240 mIU/mL on 12 March.", nil},
		{"sonologist", "Procedure conducted by the sonologist on duty.", nil},
		{"ultrasonography", "Obstetric ultrasonography, transabdominal.", nil},
		{"reason", "Reason for the scan: estimation of gestational age.", nil},
		{"person", "The person accompanying the patient waited outside.", nil},
		{"empty", "", nil},
		{"only punctuation", "--", nil},
		{"season", "Seasonal fever three weeks earlier.", nil},
		{"mason", "Patient's occupation: mason.", nil},
		{"unisex is not a whole word match", "Clinic shares a unisex waiting area.", nil},

		// The terms that have no other use in a scan note.
		{"plain English", "sex", []string{"sex"}},
		{"capitalised", "SEX", []string{"sex"}},
		{"mid sentence", "Patient asked about the sex.", []string{"sex"}},
		{"gender", "Family enquired about gender.", []string{"gender"}},
		{"male", "Told them male.", []string{"male"}},
		{"female", "Noted female.", []string{"female"}},
		{"boy", "They want a boy.", []string{"boy"}},
		{"girls", "Two girls already at home.", []string{"girls"}},
		{"daughter", "Has one daughter.", []string{"daughter"}},
		{"sons", "Two sons living.", []string{"sons"}},

		// Karyotype shorthand says the same thing in two characters, which is
		// exactly why an English-only word list is not enough.
		{"karyotype XY", "Result 46,XY.", []string{"xy"}},
		{"lowercase karyotype", "result 46,xy", []string{"xy"}},
		{"karyotype XX with the count in front", "Karyotype 46, XX reported.", []string{"46,xx"}},
		{"karyotype XX, no space", "46,XX", []string{"46,xx"}},

		// The form numbers its twentieth indication "(xx)", so the other
		// half of the karyotype shorthand is matched only in the form a
		// karyotype is written in. These two cases pin that trade-off.
		{"the form's own numbering is not a karyotype", "Indication (xx) applies — adjunct to an invasive procedure.", nil},
		{"a bare XX is a known and deliberate hole", "xx", nil},

		// A note in a clinic is as likely to be in Hindi or Marathi.
		{"ladki", "Pehle se do ladki hain.", []string{"ladki"}},
		{"ladka", "Ladka chahiye.", []string{"ladka"}},
		{"beti", "Ek beti hai.", []string{"beti"}},
		{"mulgi", "Doghi mulgi aahet.", []string{"mulgi"}},
		{"putri", "Putri janm 2019.", []string{"putri"}},

		// Several terms at once: every one is named, deduplicated, in the
		// order the text puts them, so the message can list them all.
		{"several terms", "boy or girl, boy preferred", []string{"boy", "girl"}},
		{"mixed scripts and case", "Gender? 46,XY — ladka", []string{"gender", "xy", "ladka"}},
		{"repeated single term", "sex sex sex", []string{"sex"}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := ScreenText(c.in)
			if !reflect.DeepEqual(got, c.want) {
				t.Errorf("ScreenText(%q) = %v, want %v", c.in, got, c.want)
			}
			if TextIsClear(c.in) != (len(c.want) == 0) {
				t.Errorf("TextIsClear(%q) = %v, which contradicts ScreenText", c.in, TextIsClear(c.in))
			}
		})
	}
}

func TestScreenedTermsIsNotEmptyAndCannotBeMutatedByCallers(t *testing.T) {
	first := ScreenedTerms()
	if len(first) == 0 {
		t.Fatal("ScreenedTerms() is empty: the screening vocabulary has gone missing")
	}
	first[0] = "mutated"
	second := ScreenedTerms()
	if second[0] == "mutated" {
		t.Error("ScreenedTerms() hands out the live vocabulary; a caller could disable the guard")
	}
	if !TextIsClear("mutated") {
		t.Error("mutating the returned slice changed what the guard screens for")
	}
}

// TestScreenedWordsAreWholeWordsOnly guards the guard: every entry in the
// vocabulary must be a plain lowercase word, because the pattern is built by
// joining them into one alternation and a stray metacharacter would silently
// change what is matched.
func TestScreenedWordsAreWholeWordsOnly(t *testing.T) {
	for _, w := range screenedWords {
		if w == "" {
			t.Error("empty entry in the screening vocabulary")
			continue
		}
		for _, r := range w {
			if (r < 'a' || r > 'z') && (r < '0' || r > '9') {
				t.Errorf("screened word %q contains %q; entries must be plain lowercase words", w, r)
			}
		}
		if !reflect.DeepEqual(ScreenText(w), []string{w}) {
			t.Errorf("screened word %q is not matched by the pattern built from it", w)
		}
	}
}

// TestScreeningRefusesTheWholeRecordNotJustOneField is the behaviour that
// matters: the refusal names each offending field and each term in it.
func TestScreeningRefusesTheWholeRecordNotJustOneField(t *testing.T) {
	r := completeRecord()
	r.Result = "Single live intrauterine pregnancy. Family asked about the sex."
	r.Remarks = "Already has two girls."

	v := Validate(r)
	refused := v.Refused()
	if len(refused) != 2 {
		t.Fatalf("Validate refused %d fields, want 2 (result and remarks); issues: %v", len(refused), v.Issues)
	}
	if v.Complete() {
		t.Error("a record carrying screened terms reported itself complete")
	}
	if v.Storable() {
		t.Error("a record carrying screened terms reported itself storable")
	}
	wantFields := map[string]string{"result": "sex", "remarks": "girls"}
	for _, iss := range refused {
		term, ok := wantFields[iss.Field]
		if !ok {
			t.Errorf("unexpected refused field %q", iss.Field)
			continue
		}
		if !contains(iss.Terms, term) {
			t.Errorf("refusal for %q names terms %v, want it to name %q", iss.Field, iss.Terms, term)
		}
		delete(wantFields, iss.Field)
	}
	for f := range wantFields {
		t.Errorf("field %q carried a screened term and was not refused", f)
	}
}

func contains(list []string, want string) bool {
	for _, s := range list {
		if s == want {
			return true
		}
	}
	return false
}
