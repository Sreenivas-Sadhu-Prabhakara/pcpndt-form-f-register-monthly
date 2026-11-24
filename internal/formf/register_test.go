package formf

import (
	"reflect"
	"strings"
	"testing"
	"time"
)

// TestPrintableRegisterUsesTheStatutoryColumnOrder pins the columns of the
// register rule 9(1) prescribes: serial order, her name and address, her
// spouse's or father's name, and the date she first reported — in that order,
// each marked with what asks for it.
func TestPrintableRegisterUsesTheStatutoryColumnOrder(t *testing.T) {
	asOf := Day(2026, time.December, 2)
	got := PrintableRegister(testClinic(), []Record{completeRecord()}, testSettings(), asOf)

	var keys, sources []string
	for _, c := range got.Columns {
		keys = append(keys, c.Key)
		sources = append(sources, c.Source)
		if c.Heading == "" {
			t.Errorf("column %q has no heading", c.Key)
		}
	}
	wantKeys := []string{"serial", "patientName", "address", "relativeName", "firstReportedOn", "procedureDate", "formF", "retainUntil"}
	if !reflect.DeepEqual(keys, wantKeys) {
		t.Errorf("columns = %v, want %v", keys, wantKeys)
	}
	wantSources := []string{"rule 9(1)", "rule 9(1)", "rule 9(1)", "rule 9(1)", "rule 9(1)", "Form F item 13", "this register", "this register"}
	if !reflect.DeepEqual(sources, wantSources) {
		t.Errorf("column sources = %v, want %v", sources, wantSources)
	}

	if len(got.Rows) != 1 {
		t.Fatalf("%d rows, want 1", len(got.Rows))
	}
	row := got.Rows[0]
	if len(row.Cells) != len(got.Columns) {
		t.Fatalf("%d cells against %d columns", len(row.Cells), len(got.Columns))
	}
	wantCells := []string{"1", "Shalini Kamath", "12 Gokhale Path, Dharwad 580001, Karnataka", "Rohit Kamath", "2026-11-16", "2026-11-18", "complete", "2028-11-18"}
	if !reflect.DeepEqual(row.Cells, wantCells) {
		t.Errorf("cells = %v\nwant       %v", row.Cells, wantCells)
	}
	if !row.Complete {
		t.Error("a complete entry printed as incomplete")
	}
	if got.Incomplete != nil {
		t.Errorf("Incomplete = %v on a clean register", got.Incomplete)
	}
}

func TestPrintableRegisterOrdersBySerialAndFlagsWhatIsWrong(t *testing.T) {
	asOf := Day(2026, time.December, 2)
	broken := recordOn(2, "2026-11-04")
	broken.Place = ""
	pending := recordOn(4, "2026-11-06")
	pending.ProceedingsPending = true

	records := []Record{pending, recordOn(1, "2026-11-03"), broken}
	got := PrintableRegister(testClinic(), records, testSettings(), asOf)

	var serials []int
	for _, r := range got.Rows {
		serials = append(serials, r.Serial)
	}
	if want := []int{1, 2, 4}; !reflect.DeepEqual(serials, want) {
		t.Errorf("rows in order %v, want %v", serials, want)
	}
	if !reflect.DeepEqual(got.Incomplete, []int{2}) {
		t.Errorf("Incomplete = %v, want [2]", got.Incomplete)
	}
	if got.Serial.Intact {
		t.Error("a register missing serial 3 printed as intact")
	}
	if got.Rows[1].Cells[6] == "complete" {
		t.Error("an entry with a blank item printed as complete")
	}
	if !strings.HasSuffix(got.Rows[2].Cells[7], "or later") {
		t.Errorf("an entry with proceedings noted printed a plain release date: %q", got.Rows[2].Cells[7])
	}
	if !got.Rows[2].OpenEnded {
		t.Error("OpenEnded not set on the entry with proceedings noted")
	}
	if got.SampleSize != 3 {
		t.Errorf("SampleSize = %d, want 3 for a three-entry register", got.SampleSize)
	}
}

func TestPrintableRegisterCarriesItsAuthenticationAndItsNotes(t *testing.T) {
	asOf := Day(2026, time.December, 2)
	got := PrintableRegister(testClinic(), []Record{completeRecord()}, testSettings(), asOf)

	if !strings.Contains(got.Authentication.Reference, "rule 9(7)") {
		t.Errorf("the authentication block does not cite rule 9(7): %q", got.Authentication.Reference)
	}
	if got.Authentication.Person != "Dr M. Rao" {
		t.Errorf("Authentication.Person = %q, want the responsible person from settings", got.Authentication.Person)
	}
	if !strings.Contains(got.Authentication.Statement, "2026-12-02") {
		t.Errorf("the authentication statement does not carry the date printed: %q", got.Authentication.Statement)
	}

	// With nobody named, the block has to say so rather than print a gap.
	noPerson := PrintableRegister(testClinic(), nil, DefaultSettings(), asOf)
	if !strings.Contains(noPerson.Authentication.Person, "name the person responsible") {
		t.Errorf("with no responsible person set, Person = %q", noPerson.Authentication.Person)
	}

	notes := strings.Join(got.Footnotes, "\n")
	for _, want := range []string{
		"rule 9(7)", "rule 9(6)", "whichever is later", "whichever is more",
		"Not held in this register — 4(a) and 4(b)",
		"Section C, items 17 to 27",
		"files nothing",
	} {
		if !strings.Contains(notes, want) {
			t.Errorf("the register's notes do not mention %q", want)
		}
	}

	// A changed statutory figure has to be declared on the page.
	changed := testSettings()
	changed.RetentionYears = 5
	withOverride := PrintableRegister(testClinic(), nil, changed, asOf)
	if !strings.Contains(strings.Join(withOverride.Footnotes, "\n"), "changed from this tool's indicative defaults") {
		t.Error("a register with an overridden retention window does not say so in its notes")
	}
}

func TestFormFaceMarksEveryBlankAgainstItsItemNumber(t *testing.T) {
	r := completeRecord()
	r.Place = ""
	r.SealAffixed = false

	face := FormFace(r)
	if len(face) != len(Fields()) {
		t.Fatalf("the face has %d lines against %d fields", len(face), len(Fields()))
	}

	byLabel := map[string]FaceLine{}
	marked := map[string]bool{}
	for _, l := range face {
		byLabel[l.Label] = l
		if l.Missing {
			marked[l.Label] = true
		}
	}
	if len(marked) != 2 {
		t.Errorf("%d lines marked missing, want 2: %v", len(marked), marked)
	}
	if !marked["Place"] || !marked["Seal affixed on the signed form"] {
		t.Errorf("the wrong lines are marked: %v", marked)
	}
	if l := byLabel["Her name"]; l.Value != "Shalini Kamath" || l.Missing {
		t.Errorf("a filled line reads %+v", l)
	}
	if l := byLabel["Date the procedure was carried out"]; l.Item != "13" {
		t.Errorf("the procedure date carries item %q, want 13", l.Item)
	}

	// The groups come out in the order the paperwork works through them,
	// and each line carries the help text that says what the item wants.
	var groups []string
	for _, l := range face {
		if len(groups) == 0 || groups[len(groups)-1] != l.Group {
			groups = append(groups, l.Group)
		}
		if l.Help == "" {
			t.Errorf("line %q has no help text", l.Label)
		}
	}
	want := []string{GroupRule9Register, GroupSectionA, GroupSectionB, GroupSignature, GroupSectionD, GroupLocal}
	if !reflect.DeepEqual(groups, want) {
		t.Errorf("groups came out as %v, want %v", groups, want)
	}
}

func TestRenderedPagesSayWhatTheyHaveTo(t *testing.T) {
	asOf := Day(2026, time.December, 2)
	m, err := ParseMonth("2026-11")
	if err != nil {
		t.Fatal(err)
	}
	records := []Record{recordOn(1, "2026-11-03"), recordOn(2, "2026-11-04")}
	tallies := []ProcedureTally{{Date: "2026-11-03", Count: 1}, {Date: "2026-11-04", Count: 1}}

	t.Run("the register", func(t *testing.T) {
		out := RenderRegisterText(PrintableRegister(testClinic(), records, testSettings(), asOf))
		for _, want := range []string{
			"REGISTER OF PROCEDURES — RULE 9(1)",
			"Sahyadri Imaging Centre",
			"PCPNDT/DHW/2019/0147",
			"statutory column order",
			"[rule 9(1)]",
			"Serials 1 to 2, unbroken, none repeated.",
			"RETENTION",
			"2 of these 2 entries would be scrutinised",
			"AUTHENTICATION",
			"Signature ____",
			"WHAT THIS REGISTER WILL NOT HOLD",
			"NOTES",
		} {
			if !strings.Contains(out, want) {
				t.Errorf("the printed register does not contain %q", want)
			}
		}
	})

	t.Run("the monthly report", func(t *testing.T) {
		out := RenderReturnText(AssembleReturn(testClinic(), records, tallies, m, testSettings(), asOf))
		for _, want := range []string{
			"MONTHLY REPORT OF PROCEDURES — NOVEMBER 2026",
			"District Appropriate Authority, Dharwad",
			"Due by          : 2026-12-05",
			"3 day(s) to the due date",
			"rule 9(8)",
			"prescribes no format",
			"BY INDICATION — ITEM 10",
			"DAY BY DAY — FORM F AGAINST THE CENTRE'S OWN COUNT",
			"ENTRIES IN THIS REPORT",
			"READY TO SEND",
			"This tool files nothing.",
		} {
			if !strings.Contains(out, want) {
				t.Errorf("the printed report does not contain %q", want)
			}
		}
	})

	t.Run("a report that is not ready", func(t *testing.T) {
		broken := []Record{recordOn(1, "2026-11-03"), func() Record { r := recordOn(3, "2026-11-04"); r.Place = ""; return r }()}
		out := RenderReturnText(AssembleReturn(testClinic(), broken, tallies, m, testSettings(), asOf))
		if strings.Contains(out, "READY TO SEND\n  Copy this") {
			t.Error("a report with blanks and a numbering gap printed as ready to send")
		}
		if !strings.Contains(out, "NOT READY TO SEND") {
			t.Error("a report with blanks does not say it is not ready")
		}
	})

	t.Run("one Form F face", func(t *testing.T) {
		out := RenderFormFaceText(testClinic(), records[0], testSettings(), asOf)
		for _, want := range []string{
			"FORM F — ENTRY 1",
			"[see proviso to s.4(3), rule 9(4) and rule 10(1A)] — Sections A, B and D",
			"Item 1  Centre           : Sahyadri Imaging Centre",
			"Item 2  Registration no. : PCPNDT/DHW/2019/0147",
			strings.ToUpper(GroupSectionB),
			"COMPLETENESS",
			"Complete: all 30 applicable items are filled in.",
			"RETENTION",
			"not a substitute for them",
		} {
			if !strings.Contains(out, want) {
				t.Errorf("the printed Form F face does not contain %q", want)
			}
		}
	})

	t.Run("an empty register still prints", func(t *testing.T) {
		out := RenderRegisterText(PrintableRegister(Clinic{}, nil, DefaultSettings(), asOf))
		if !strings.Contains(out, "(no entries)") {
			t.Error("an empty register does not say it is empty")
		}
		if !strings.Contains(out, "No entries yet.") {
			t.Error("an empty register does not say so under numbering")
		}
		if strings.Contains(out, "would be scrutinised") {
			t.Error("an empty register claims an inspection sample")
		}
	})
}

func TestWorkingIndexHoldsEveryStoredField(t *testing.T) {
	asOf := Day(2026, time.December, 2)
	got := BuildWorkingIndex(testClinic(), []Record{recordOn(2, "2026-11-04"), recordOn(1, "2026-11-03")}, testSettings(), asOf)

	if len(got.Columns) != len(Fields()) {
		t.Errorf("%d columns against %d fields", len(got.Columns), len(Fields()))
	}
	var serials []int
	for _, r := range got.Rows {
		serials = append(serials, r.Serial)
		if len(r.Cells) != len(got.Columns) {
			t.Errorf("entry %d has %d cells against %d columns", r.Serial, len(r.Cells), len(got.Columns))
		}
	}
	if want := []int{1, 2}; !reflect.DeepEqual(serials, want) {
		t.Errorf("rows in order %v, want %v", serials, want)
	}

	// Each column says what asks for it, so that nothing on this page can
	// be mistaken for a prescribed column.
	bySource := map[string]int{}
	for _, c := range got.Columns {
		bySource[c.Source]++
	}
	if bySource["this register"] != 3 {
		t.Errorf("%d columns attributed to this register, want 3", bySource["this register"])
	}
	if bySource["rule 9(1)"] != 2 {
		t.Errorf("%d columns attributed to rule 9(1), want 2", bySource["rule 9(1)"])
	}
	if bySource["Form F 13"] != 1 {
		t.Errorf("the procedure date column is attributed to %v", bySource)
	}
}

func TestTextHelpers(t *testing.T) {
	t.Run("clip", func(t *testing.T) {
		cases := []struct {
			in   string
			n    int
			want string
		}{
			{"", 5, ""},
			{"short", 10, "short"},
			{"exactly10!", 10, "exactly10!"},
			{"eleven chars", 10, "eleven ch…"},
			// Counted in runes, not bytes, so a multi-byte name is
			// not cut through the middle of a character.
			{"αβγδεζη", 5, "αβγδ…"},
			{"Dhārwād — Karnātaka", 8, "Dhārwād…"},
			{"abc", 1, "a"},
		}
		for _, c := range cases {
			if got := clip(c.in, c.n); got != c.want {
				t.Errorf("clip(%q, %d) = %q, want %q", c.in, c.n, got, c.want)
			}
		}
	})

	t.Run("oneLine", func(t *testing.T) {
		if got := oneLine("12 Gokhale Path,\n Dharwad\t580001\n"); got != "12 Gokhale Path, Dharwad 580001" {
			t.Errorf("oneLine = %q", got)
		}
		if got := oneLine(""); got != "" {
			t.Errorf("oneLine of an empty string = %q", got)
		}
	})

	t.Run("orDash", func(t *testing.T) {
		if orDash("") != "—" || orDash("  ") != "—" || orDash("x") != "x" {
			t.Error("orDash does not replace blanks with a dash")
		}
	})

	t.Run("wrapIndent", func(t *testing.T) {
		got := wrapIndent("one two three four five six seven", 12, "..")
		want := "one two\n..three four\n..five six\n..seven"
		if got != want {
			t.Errorf("wrapIndent =\n%q\nwant\n%q", got, want)
		}
		if wrapIndent("", 10, "  ") != "" {
			t.Error("wrapIndent of an empty string is not empty")
		}
	})

	t.Run("joinInts", func(t *testing.T) {
		if got := joinInts([]int{3, 1, 2}); got != "3, 1, 2" {
			t.Errorf("joinInts = %q", got)
		}
		if got := joinInts(nil); got != "" {
			t.Errorf("joinInts(nil) = %q", got)
		}
	})

	t.Run("joinIndications numbers them as the form does", func(t *testing.T) {
		got := joinIndications([]string{indicationNum("(ii)"), IndOther})
		if !strings.HasPrefix(got, "(ii) Estimation of gestational age") {
			t.Errorf("joinIndications = %q, want it to lead with the form's number", got)
		}
		if strings.Contains(got, "— An indication not") {
			t.Errorf("joinIndications printed a dash as a number: %q", got)
		}
	})
}

func TestIndicationLookup(t *testing.T) {
	all := Indications()
	if len(all) != 24 {
		t.Errorf("%d indications, want the 23 the form prints plus the one this register adds", len(all))
	}
	listed := 0
	nums := map[string]bool{}
	for _, i := range all {
		if i.Listed {
			listed++
		}
		if i.Text == "" || i.Num == "" {
			t.Errorf("indication %+v is incomplete", i)
		}
		if nums[i.Num] {
			t.Errorf("two indications numbered %q", i.Num)
		}
		nums[i.Num] = true
		if IndicationNumber(i.Text) != i.Num {
			t.Errorf("IndicationNumber(%q) = %q, want %q", clip(i.Text, 30), IndicationNumber(i.Text), i.Num)
		}
	}
	if listed != 23 {
		t.Errorf("%d listed indications, want 23", listed)
	}
	if !IsListedIndication(indicationNum("(i)")) {
		t.Error("the form's first indication is not recognised as listed")
	}
	if IsListedIndication(IndOther) {
		t.Error("the entry this register adds is reported as one the form prints")
	}
	if IsListedIndication("something made up") {
		t.Error("made-up wording is reported as a listed indication")
	}
	if IndicationNumber("something made up") != "" {
		t.Error("made-up wording was given a number")
	}
	if got := CountListedIndications([]string{indicationNum("(i)"), IndOther, "made up"}); got != 1 {
		t.Errorf("CountListedIndications = %d, want 1", got)
	}
	if len(IndicationTexts()) != len(all) {
		t.Error("IndicationTexts and Indications disagree on how many there are")
	}
}
