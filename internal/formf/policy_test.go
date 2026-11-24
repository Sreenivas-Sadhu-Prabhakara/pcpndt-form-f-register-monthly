package formf

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"
	"time"
)

// What this file is for.
//
// The Act this register serves exists to stop sex selection. The tool
// therefore has to be built so that it has nowhere to put what the Act
// prohibits — and "has to be built so" is a claim a comment cannot make good.
// So this file makes it mechanically.
//
// scanned walks a surface of the package and reports anything matching the
// screening vocabulary in screening.go. The surfaces are: every field in the
// catalogue, with its key, item number, group, label, column heading, help
// text, conditional note and every choice it offers; every column heading of
// every page this tool prints; every line of the printed register, the printed
// working index, the printed Form F face and the printed monthly report; every
// key in the JSON a record serialises to; every field name of every exported
// struct; and every message the checker produces from an entry that did not
// itself contain such a term.
//
// Two things are deliberately outside the scan, and both are asserted
// separately below.
//
// The first is PolicyStatement, which names the subject on purpose, because
// the tool is required to say plainly what it does not do. TestPolicyStatement
// asserts it is present and that it still says it.
//
// The second is a refusal message, which echoes back the term the user typed
// in order to name what has to be removed. That is the tool repeating an input,
// not originating a field, and TestRefusalNamesTheTermTheUserTyped asserts it
// happens. Everything the tool generates by itself is in the scan.

// scanned returns every screened term found in s, along with the text, so a
// failure message can point at the offending surface.
func scanned(t *testing.T, where, s string) {
	t.Helper()
	if terms := ScreenText(s); len(terms) > 0 {
		t.Errorf("%s names %v — this register must have no field, label, heading or output that does:\n  %q", where, terms, clip(s, 300))
	}
}

func TestNoFieldLabelOrHeadingNamesTheSubject(t *testing.T) {
	for _, f := range Fields() {
		where := "field " + f.Key
		scanned(t, where+" key", f.Key)
		scanned(t, where+" item", f.Item)
		scanned(t, where+" group", f.Group)
		scanned(t, where+" label", f.Label)
		scanned(t, where+" column heading", f.Column)
		scanned(t, where+" help text", f.Help)
		scanned(t, where+" condition", f.When)
		for _, c := range f.Choices {
			scanned(t, where+" choice", c)
		}
	}
	for _, g := range []string{GroupRule9Register, GroupSectionA, GroupSectionB, GroupSignature, GroupSectionD, GroupLocal} {
		scanned(t, "group name", g)
	}
}

func TestNoSelectableValueNamesTheSubject(t *testing.T) {
	for _, i := range Indications() {
		scanned(t, "indication "+i.Num, i.Text)
		scanned(t, "indication number", i.Num)
	}
	for _, p := range Procedures() {
		scanned(t, "procedure", p)
	}
	for _, a := range MTPAnswers() {
		scanned(t, "item 16 answer", a)
	}
	for _, tech := range invasiveTechniques {
		scanned(t, "invasive technique", tech)
	}
}

func TestNoStoredFieldNamesTheSubject(t *testing.T) {
	// The JSON a record serialises to is what ends up in the data file and
	// what the interface reads. No key in it may name the subject.
	b, err := json.Marshal(completeRecord())
	if err != nil {
		t.Fatalf("marshalling a record: %v", err)
	}
	var asMap map[string]any
	if err := json.Unmarshal(b, &asMap); err != nil {
		t.Fatalf("unmarshalling a record: %v", err)
	}
	if len(asMap) < 30 {
		t.Fatalf("a record serialised to only %d keys; the scan would be checking almost nothing", len(asMap))
	}
	for k := range asMap {
		scanned(t, "data-file key", k)
	}

	// And no Go field name of any exported type, because a field name is
	// the first thing anyone reads.
	for _, v := range []any{
		Record{}, Clinic{}, Settings{}, Field{}, Issue{}, Validation{},
		Retention{}, RetentionSummary{}, SerialCheck{}, SerialIssue{},
		ProcedureTally{}, DayReconciliation{}, MonthCheck{}, MonthlyReturn{},
		ReturnTotals{}, Count{}, PrintedRegister{}, WorkingIndex{},
		RegisterColumn{}, RegisterRow{}, FaceLine{}, Authentication{},
		Indication{}, ItemNotStored{}, Month{},
	} {
		ty := reflect.TypeOf(v)
		for i := 0; i < ty.NumField(); i++ {
			scanned(t, ty.Name()+"."+ty.Field(i).Name, ty.Field(i).Name)
			scanned(t, ty.Name()+" json tag", ty.Field(i).Tag.Get("json"))
		}
	}
}

func TestNoPrintedOutputNamesTheSubject(t *testing.T) {
	asOf := Day(2026, time.December, 2)
	m, err := ParseMonth("2026-11")
	if err != nil {
		t.Fatal(err)
	}

	// A populated register, an entry with a thumb impression, an entry
	// with an indication for a termination, and an incomplete entry —
	// between them these exercise every conditional branch of every page.
	thumb := recordOn(2, "2026-11-04")
	thumb.ThumbImpression = true
	thumb.IdentifierName = "Savitri Bai"
	thumb.IdentifierAge = 52
	thumb.IdentifierRelation = "Mother-in-law"
	thumb.IdentifierAddress = "12 Gokhale Path, Dharwad — 9876500001"
	thumb.AttestedBy = "Dr M. Rao"
	thumb.AttestedOn = "2026-11-04"

	mtp := recordOn(3, "2026-11-05")
	mtp.MTPIndication = MTPIndicated
	mtp.MTPDetail = "Anomaly seen on the anomaly scan; referred for a second opinion."

	broken := recordOn(5, "2026-11-06")
	broken.Place = ""
	broken.SealAffixed = false
	broken.ProceedingsPending = true

	selfRef := recordOn(6, "2026-11-09")
	selfRef.SelfReferral = true
	selfRef.ReferrerName, selfRef.ReferrerAddress, selfRef.ReferrerQualificatn = "", "", ""

	records := []Record{recordOn(1, "2026-11-03"), thumb, mtp, broken, selfRef}
	tallies := []ProcedureTally{
		{Date: "2026-11-03", Count: 2, Source: "OPD register"},
		{Date: "2026-11-04", Count: 1, Source: "OPD register"},
	}

	// Each page, on populated data and on an empty register.
	for _, c := range []struct {
		name string
		recs []Record
		tals []ProcedureTally
	}{
		{"a populated register", records, tallies},
		{"an empty register", nil, nil},
	} {
		t.Run(c.name, func(t *testing.T) {
			reg := PrintableRegister(testClinic(), c.recs, testSettings(), asOf)
			// The policy statement names the subject on purpose, and
			// is asserted on its own in TestPolicyStatementSaysItPlainly.
			// Blanking it leaves the rest of the page — every heading,
			// every cell, every note, every finding — in the scan.
			forScan := reg
			forScan.Policy = ""
			scanned(t, "printed register", RenderRegisterText(forScan))
			for _, col := range reg.Columns {
				scanned(t, "register column", col.Heading+" "+col.Source)
			}
			for _, n := range reg.Footnotes {
				scanned(t, "register footnote", n)
			}
			scanned(t, "register authentication", reg.Authentication.Statement+" "+reg.Authentication.Reference+" "+reg.Authentication.SignatureL)

			idx := BuildWorkingIndex(testClinic(), c.recs, testSettings(), asOf)
			for _, col := range idx.Columns {
				scanned(t, "working index column", col.Heading+" "+col.Source)
			}
			for _, row := range idx.Rows {
				scanned(t, "working index row", strings.Join(row.Cells, " | "))
			}

			ret := AssembleReturn(testClinic(), c.recs, c.tals, m, testSettings(), asOf)
			scanned(t, "monthly report", RenderReturnText(ret))
			for _, blk := range ret.Blockers {
				scanned(t, "report blocker", blk)
			}
			for _, f := range ret.Check.Findings {
				scanned(t, "month finding", f)
			}
		})
	}

	for _, r := range records {
		scanned(t, "printed Form F face", RenderFormFaceText(testClinic(), r, testSettings(), asOf))
		for _, l := range FormFace(r) {
			scanned(t, "Form F face line", l.Group+" "+l.Item+" "+l.Label+" "+l.Help+" "+l.Value)
		}
	}
}

func TestNoGeneratedMessageNamesTheSubject(t *testing.T) {
	// Messages from entries that carry nothing of the sort themselves. If
	// the checker's own prose named the subject it would show up here.
	for _, r := range []Record{
		{},
		completeRecord(),
		func() Record { r := completeRecord(); r.LivingChildren = intPtr(0); return r }(),
		func() Record { r := completeRecord(); r.ThumbImpression = true; return r }(),
		func() Record { r := completeRecord(); r.SelfReferral = true; return r }(),
		func() Record { r := completeRecord(); r.MTPIndication = MTPIndicated; return r }(),
		func() Record {
			r := completeRecord()
			r.Procedures = []string{ProcOther}
			r.ProcedureOther = "Amniocentesis"
			return r
		}(),
		func() Record { r := completeRecord(); r.ProcedureDate = "18-11-2026"; return r }(),
		func() Record { r := completeRecord(); r.Indications = []string{"something off-list"}; return r }(),
	} {
		v := ValidateOn(r, Day(2026, time.December, 2))
		scanned(t, "validation summary", v.Summary)
		for _, iss := range v.Issues {
			scanned(t, "issue message", iss.Message)
			scanned(t, "issue label", iss.Label)
		}
	}

	for _, recs := range [][]Record{
		nil,
		entries(1, "2026-11-01", 4, "2026-11-04"),
		entries(0, "2026-11-01", 1, "2026-11-01", 1, "2026-11-02"),
		entries(2, "2026-11-05", 3, "2026-11-01"),
	} {
		for _, iss := range CheckSerials(recs, DefaultSettings()).Issues {
			scanned(t, "numbering message", iss.Message)
		}
	}

	for _, n := range ItemsNotStored() {
		scanned(t, "not-stored item", n.Item)
		scanned(t, "not-stored asks", n.Asks)
		scanned(t, "not-stored reason", n.Reason)
		scanned(t, "not-stored where", n.Where)
	}
	for _, ref := range []string{
		RetentionReference, RetentionProviso, ReturnDueReference, ReturnFormatNote,
		PrintedCopyReference, InspectionSampleReference,
	} {
		scanned(t, "statutory reference", ref)
	}
}

// TestPolicyStatementSaysItPlainly is the other half of the bargain. The tool
// is required to state, in the interface and in the documentation, that it
// does not and will not hold this. A statement that said nothing would satisfy
// every scan above and none of the obligation, so it is asserted directly.
func TestPolicyStatementSaysItPlainly(t *testing.T) {
	p := PolicyStatement
	for _, want := range []string{
		"does not record",
		"sex of a foetus",
		"There is no field for it",
		"there never will be",
		"refused",
		"item 4",
	} {
		if !strings.Contains(p, want) {
			t.Errorf("PolicyStatement no longer says %q:\n%s", want, p)
		}
	}
	if len(p) < 400 {
		t.Errorf("PolicyStatement is %d characters; it has a lot to say and should say it", len(p))
	}
	// It has to reach the pages, or it is a constant nobody reads.
	asOf := Day(2026, time.December, 2)
	reg := PrintableRegister(testClinic(), []Record{completeRecord()}, testSettings(), asOf)
	if reg.Policy != p {
		t.Error("the printed register does not carry the policy statement")
	}
	if !strings.Contains(RenderRegisterText(reg), "WHAT THIS REGISTER WILL NOT HOLD") {
		t.Error("the printed register has no section stating what it will not hold")
	}
	if BuildWorkingIndex(testClinic(), nil, testSettings(), asOf).Policy != p {
		t.Error("the working index does not carry the policy statement")
	}
}

// TestRefusalNamesTheTermTheUserTyped is the one case where the tool repeats
// the word: in refusing it. The refusal has to be specific or the clinic
// cannot act on it.
func TestRefusalNamesTheTermTheUserTyped(t *testing.T) {
	r := completeRecord()
	r.Result = "Single live intrauterine pregnancy. Family asked the sex; not discussed."

	v := Validate(r)
	if v.Storable() {
		t.Fatal("an entry carrying a checked term reported itself storable")
	}
	refused := v.Refused()
	if len(refused) != 1 {
		t.Fatalf("%d fields refused, want 1", len(refused))
	}
	iss := refused[0]
	if iss.Field != "result" {
		t.Errorf("refused field = %q, want result", iss.Field)
	}
	if !reflect.DeepEqual(iss.Terms, []string{"sex"}) {
		t.Errorf("Terms = %v, want [sex]", iss.Terms)
	}
	if !strings.Contains(iss.Message, `"sex"`) {
		t.Errorf("the refusal does not quote the term the user typed: %q", iss.Message)
	}
	for _, want := range []string{"cannot be stored", "no record of what the Act prohibits", "Rewrite it"} {
		if !strings.Contains(iss.Message, want) {
			t.Errorf("the refusal does not say %q: %q", want, iss.Message)
		}
	}
}

// TestEveryFreeTextFieldIsScreened guards against a new free-text field being
// added without screening. Anything that can hold a sentence must be checked.
func TestEveryFreeTextFieldIsScreened(t *testing.T) {
	// The long-text fields and the ones that invite a phrase. Names,
	// addresses, dates, numbers and fixed choices are deliberately not
	// screened: there is nothing to encode in a date, and a surname or a
	// place name could collide with the vocabulary.
	notScreenedOnPurpose := map[string]bool{
		"address":            true, // an address
		"referrerAddress":    true, // an address
		"identifierAddress":  true, // an address
		"identifierRelation": true, // a relation, which has no clinical content
		"conveyedTo":         true, // a person
	}
	for _, f := range Fields() {
		if f.Kind != KindLongText && f.Key != "childAges" && f.Key != "procedureOther" {
			continue
		}
		if notScreenedOnPurpose[f.Key] {
			if f.Screened {
				t.Errorf("%s is screened but is listed as deliberately unscreened; the list and the catalogue disagree", f.Key)
			}
			continue
		}
		if !f.Screened {
			t.Errorf("%s holds free text and is not screened", f.Key)
		}
	}

	// And the screening actually runs on each of them.
	for _, f := range Fields() {
		if !f.Screened {
			continue
		}
		r := completeRecord()
		switch f.Key {
		case "childAges":
			r.ChildAges = "one girl, 4 y"
		case "indicationDetail":
			r.IndicationDetail = "Referral asks about the sex"
		case "procedureOther":
			r.Procedures = []string{ProcOther}
			r.ProcedureOther = "scan to tell the gender"
		case "result":
			r.Result = "Normal scan. Male."
		case "mtpDetail":
			r.MTPIndication = MTPIndicated
			r.MTPDetail = "Family wants a boy"
		case "remarks":
			r.Remarks = "46,XY"
		default:
			t.Errorf("no screening case written for the screened field %q", f.Key)
			continue
		}
		v := Validate(r)
		if v.Storable() {
			t.Errorf("%s accepted text naming the subject", f.Key)
		}
		found := false
		for _, iss := range v.Refused() {
			if iss.Field == f.Key {
				found = true
			}
		}
		if !found {
			t.Errorf("%s was not the field refused; issues: %+v", f.Key, v.Refused())
		}
	}
}
