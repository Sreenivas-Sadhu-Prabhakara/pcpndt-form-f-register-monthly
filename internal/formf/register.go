package formf

import (
	"fmt"
	"sort"
	"strings"
	"text/tabwriter"
	"time"
)

// The pages, as they are read rather than as they are stored.
//
// An inspection does not read a data file. It reads three things, and this
// register produces all three.
//
// The first is the register rule 9(1) prescribes: entries in serial order,
// each carrying her name and address, the name of her spouse or father, and
// the date she first reported. Those four columns and the serial are the
// statutory column order, and they are what PrintableRegister lays out.
//
// The second is the face of a particular Form F, item by item, to be read
// against the paper file. FormFace builds that, with every blank mandatory
// item marked.
//
// The third is the working index: every field this register holds, in the
// order the paperwork asks for them. It is not a prescribed document and it
// says so on its face; it exists so that the centre can see its own month at a
// glance.
//
// All three are built from the field catalogue, so that a printed page and a
// stored record cannot drift apart. And all three print, because rule 9(7)
// says that where records are kept on a computer a printed copy is taken and
// preserved after authentication by a person responsible for the record. A
// register that only existed on a screen would leave that undone.

// RegisterColumn is one column of a printed page.
type RegisterColumn struct {
	Key     string `json:"key"`
	Heading string `json:"heading"`
	// Source says what asks for the column: a rule, a Form F item, or
	// this register itself.
	Source string `json:"source"`
}

// rule9Columns are the columns rule 9(1) prescribes, in order, by catalogue
// key. The serial comes first because the rule requires the register to show
// its entries in serial order.
var rule9Columns = []string{"serial", "patientName", "address", "relativeName", "firstReportedOn"}

// Authentication is the block rule 9(7) requires at the foot of a printed
// copy of a register kept on a computer.
type Authentication struct {
	Reference  string `json:"reference"`
	Person     string `json:"person"`
	Statement  string `json:"statement"`
	SignatureL string `json:"signatureLine"`
}

func authenticationFor(c Clinic, s Settings, asOf time.Time, what string) Authentication {
	person := strings.TrimSpace(s.ResponsiblePerson)
	if person == "" {
		person = "(name the person responsible for this record in the register's settings)"
	}
	return Authentication{
		Reference: PrintedCopyReference,
		Person:    person,
		Statement: fmt.Sprintf("%s printed on %s from the register kept on the centre's own computer at %s. To be authenticated by %s, who is responsible for this record, and preserved with the centre's records.",
			what, FormatDay(asOf), orDash(c.Name), person),
		SignatureL: "Signature ____________________    Name and designation ____________________    Date ____________",
	}
}

// PrintedRegister is the rule 9(1) register, ready to print.
type PrintedRegister struct {
	Clinic         Clinic           `json:"clinic"`
	Columns        []RegisterColumn `json:"columns"`
	Rows           []RegisterRow    `json:"rows"`
	Serial         SerialCheck      `json:"serial"`
	Retention      RetentionSummary `json:"retention"`
	Incomplete     []int            `json:"incompleteSerials"`
	SampleSize     int              `json:"inspectionSample"`
	SampleRef      string           `json:"inspectionSampleReference"`
	GeneratedOn    string           `json:"generatedOn"`
	Settings       Settings         `json:"settings"`
	Authentication Authentication   `json:"authentication"`
	Footnotes      []string         `json:"footnotes"`
	Policy         string           `json:"policy"`
}

// RegisterRow is one entry as a row.
type RegisterRow struct {
	Serial      int      `json:"serial"`
	Cells       []string `json:"cells"`
	Complete    bool     `json:"complete"`
	Missing     int      `json:"missing"`
	RetainUntil string   `json:"retainUntil,omitempty"`
	OpenEnded   bool     `json:"openEnded,omitempty"`
}

// PrintableRegister lays out the register rule 9(1) prescribes: serial order,
// her name and address, her spouse's or father's name, and the date she first
// reported — followed by three columns of this register's own, marked as such,
// that an inspection makes easier rather than harder: the date of the
// procedure, whether the Form F behind the entry is complete, and the date the
// record may not be destroyed before.
func PrintableRegister(c Clinic, records []Record, s Settings, asOf time.Time) PrintedRegister {
	s = s.Normalised()

	cols := make([]RegisterColumn, 0, len(rule9Columns)+3)
	for _, key := range rule9Columns {
		f := fieldByKey(key)
		cols = append(cols, RegisterColumn{Key: f.Key, Heading: f.Column, Source: "rule 9(1)"})
	}
	proc := fieldByKey("procedureDate")
	cols = append(cols,
		RegisterColumn{Key: proc.Key, Heading: proc.Column, Source: "Form F item 13"},
		RegisterColumn{Key: "formF", Heading: "Form F", Source: "this register"},
		RegisterColumn{Key: "retainUntil", Heading: "Hold until", Source: "this register"},
	)

	ordered := make([]Record, len(records))
	copy(ordered, records)
	sort.SliceStable(ordered, func(i, j int) bool { return ordered[i].Serial < ordered[j].Serial })

	out := PrintedRegister{
		Clinic:         c,
		Columns:        cols,
		Serial:         CheckSerials(records, s),
		Retention:      SummariseRetention(records, s, asOf),
		SampleSize:     InspectionSample(len(records)),
		SampleRef:      InspectionSampleReference,
		GeneratedOn:    FormatDay(asOf),
		Settings:       s,
		Authentication: authenticationFor(c, s, asOf, "Register under rule 9(1)"),
		Policy:         PolicyStatement,
	}

	for _, r := range ordered {
		v := Validate(r)
		ret := RetentionOn(r, s, asOf)
		formF := "complete"
		if !v.Complete() {
			formF = fmt.Sprintf("%d to fix", len(v.Issues))
		}
		hold := ret.RetainUntil
		if ret.OpenEnded {
			hold = ret.RetainUntil + " or later"
		}
		cells := make([]string, 0, len(cols))
		for _, key := range rule9Columns {
			cells = append(cells, fieldByKey(key).render(r))
		}
		cells = append(cells, r.ProcedureDate, formF, hold)

		out.Rows = append(out.Rows, RegisterRow{
			Serial: r.Serial, Cells: cells,
			Complete: v.Complete(), Missing: len(v.Missing()),
			RetainUntil: ret.RetainUntil, OpenEnded: ret.OpenEnded,
		})
		if !v.Complete() {
			out.Incomplete = append(out.Incomplete, r.Serial)
		}
	}
	out.Footnotes = registerFootnotes(s)
	return out
}

// WorkingIndex is every field this register holds, for every entry, in the
// order the paperwork asks for them. It is the centre's own working view and
// not a prescribed document.
type WorkingIndex struct {
	Clinic         Clinic           `json:"clinic"`
	Columns        []RegisterColumn `json:"columns"`
	Rows           []RegisterRow    `json:"rows"`
	GeneratedOn    string           `json:"generatedOn"`
	Authentication Authentication   `json:"authentication"`
	Footnotes      []string         `json:"footnotes"`
	Policy         string           `json:"policy"`
}

// BuildWorkingIndex renders every stored field for every entry.
func BuildWorkingIndex(c Clinic, records []Record, s Settings, asOf time.Time) WorkingIndex {
	s = s.Normalised()
	cols := make([]RegisterColumn, 0, len(catalogue))
	for _, f := range catalogue {
		source := "this register"
		switch {
		case f.Item == "":
		case strings.HasPrefix(f.Item, "Rule"):
			source = strings.ToLower(f.Item)
		default:
			source = "Form F " + f.Item
		}
		cols = append(cols, RegisterColumn{Key: f.Key, Heading: f.Column, Source: source})
	}

	ordered := make([]Record, len(records))
	copy(ordered, records)
	sort.SliceStable(ordered, func(i, j int) bool { return ordered[i].Serial < ordered[j].Serial })

	out := WorkingIndex{
		Clinic: c, Columns: cols,
		GeneratedOn:    FormatDay(asOf),
		Authentication: authenticationFor(c, s, asOf, "Working index of the Form F register"),
		Footnotes:      registerFootnotes(s),
		Policy:         PolicyStatement,
	}
	for _, r := range ordered {
		cells := make([]string, 0, len(catalogue))
		for _, f := range catalogue {
			cells = append(cells, f.render(r))
		}
		v := Validate(r)
		out.Rows = append(out.Rows, RegisterRow{Serial: r.Serial, Cells: cells, Complete: v.Complete(), Missing: len(v.Missing())})
	}
	return out
}

func registerFootnotes(s Settings) []string {
	notes := []string{
		"This is a working copy kept on the centre's own computer. The signed Form F for each entry, the written consent, the referral slip and the case papers are the records the Appropriate Authority inspects; these pages index them and do not replace them.",
		PrintedCopyReference + ".",
		fmt.Sprintf("Retention applied: %d year(s) from the date each procedure was completed. %s.", s.RetentionYears, RetentionReference),
		RetentionProviso + ".",
		InspectionSampleReference + ".",
	}
	for _, n := range ItemsNotStored() {
		notes = append(notes, "Not held in this register — "+n.Item+". "+n.Asks+" "+n.Reason+" "+n.Where)
	}
	if s.Overridden() {
		notes = append(notes, "The retention window or the report due day has been changed from this tool's indicative defaults in the register's settings.")
	}
	notes = append(notes, "This tool is a record-keeping aid. It files nothing, sends nothing and connects to nothing. The requirements of your Appropriate Authority govern over anything stated here.")
	return notes
}

// FaceLine is one line of a single Form F as it reads on the page.
type FaceLine struct {
	Group   string `json:"group"`
	Item    string `json:"item,omitempty"`
	Label   string `json:"label"`
	Value   string `json:"value"`
	Missing bool   `json:"missing"`
	Help    string `json:"help,omitempty"`
}

// FormFace renders one entry item by item, in the form's order, with every
// blank mandatory item marked. This is the view to put beside the paper file.
func FormFace(r Record) []FaceLine {
	v := Validate(r)
	missing := map[string]bool{}
	for _, k := range v.MissingFields() {
		missing[k] = true
	}
	out := make([]FaceLine, 0, len(catalogue))
	for _, f := range catalogue {
		out = append(out, FaceLine{
			Group:   f.Group,
			Item:    f.Item,
			Label:   f.Label,
			Value:   f.render(r),
			Missing: missing[f.Key],
			Help:    f.Help,
		})
	}
	return out
}

// ------------------------------------------------------------ text renderings

// RenderRegisterText renders the rule 9(1) register as plain text, for a
// centre that wants a copy outside this tool or a printout from a machine with
// no browser.
func RenderRegisterText(t PrintedRegister) string {
	var b strings.Builder
	line := func(f string, a ...any) { fmt.Fprintf(&b, f+"\n", a...) }

	line("REGISTER OF PROCEDURES — RULE 9(1)")
	line("%s", strings.Repeat("=", 68))
	line("Centre          : %s", orDash(t.Clinic.Name))
	line("Registration no.: %s", orDash(t.Clinic.RegistrationNo))
	if t.Clinic.Address != "" {
		line("Address         : %s", oneLine(t.Clinic.Address))
	}
	if t.Clinic.AppropriateAuthority != "" {
		line("Authority       : %s", t.Clinic.AppropriateAuthority)
	}
	line("Entries         : %d", len(t.Rows))
	line("Printed         : %s", t.GeneratedOn)
	line("")

	line("ENTRIES — statutory column order")
	w := tabwriter.NewWriter(&b, 0, 4, 2, ' ', 0)
	heads := make([]string, 0, len(t.Columns))
	for _, c := range t.Columns {
		heads = append(heads, c.Heading)
	}
	fmt.Fprintln(w, "  "+strings.Join(heads, "\t"))
	srcs := make([]string, 0, len(t.Columns))
	for _, c := range t.Columns {
		srcs = append(srcs, "["+c.Source+"]")
	}
	fmt.Fprintln(w, "  "+strings.Join(srcs, "\t"))
	if len(t.Rows) == 0 {
		fmt.Fprintln(w, "  (no entries)")
	}
	for _, row := range t.Rows {
		cells := make([]string, len(row.Cells))
		for i, c := range row.Cells {
			cells[i] = clip(oneLine(c), 30)
		}
		fmt.Fprintln(w, "  "+strings.Join(cells, "\t"))
	}
	w.Flush()
	line("")

	line("NUMBERING")
	if t.Serial.Intact {
		if len(t.Rows) == 0 {
			line("  No entries yet.")
		} else {
			line("  Serials %d to %d, unbroken, none repeated.", t.Serial.First, t.Serial.Last)
		}
	} else {
		for _, iss := range t.Serial.Issues {
			line("  ! [%s] %s", strings.ToUpper(string(iss.Problem)), wrapIndent(iss.Message, 68, "        "))
		}
	}
	line("")

	if len(t.Incomplete) > 0 {
		line("INCOMPLETE ENTRIES")
		line("  Serial %s.", joinInts(t.Incomplete))
		line("  %s", wrapIndent("Under the proviso to section 4(3) of the Act a deficiency in the record is treated as a contravention unless the person who conducted the procedure proves otherwise. Complete these before the month's report goes in.", 68, "  "))
		line("")
	}

	line("RETENTION")
	line("  Window applied   : %d year(s) from the date of the procedure", t.Retention.Years)
	line("  Still to be held : %d", t.Retention.Holding)
	line("  Window closed    : %d", t.Retention.Releasable)
	line("  Open-ended       : %d", t.Retention.OpenEnded)
	if t.Retention.Undatable > 0 {
		line("  No usable date   : %d", t.Retention.Undatable)
	}
	if t.Retention.EarliestDue != "" {
		line("  Earliest release : %s", t.Retention.EarliestDue)
	}
	line("")

	if t.SampleSize > 0 {
		line("AT AN INSPECTION")
		line("  %d of these %d entries would be scrutinised.", t.SampleSize, len(t.Rows))
		line("")
	}

	line("AUTHENTICATION")
	line("  %s", wrapIndent(t.Authentication.Statement, 68, "  "))
	line("")
	line("  %s", t.Authentication.SignatureL)
	line("")

	if t.Policy != "" {
		line("WHAT THIS REGISTER WILL NOT HOLD")
		line("  %s", wrapIndent(t.Policy, 68, "  "))
		line("")
	}

	line("NOTES")
	for _, n := range t.Footnotes {
		line("  - %s", wrapIndent(n, 68, "    "))
	}
	return b.String()
}

// RenderFormFaceText renders one entry item by item as plain text, to be read
// against the paper form.
func RenderFormFaceText(c Clinic, r Record, s Settings, asOf time.Time) string {
	var b strings.Builder
	line := func(f string, a ...any) { fmt.Fprintf(&b, f+"\n", a...) }

	v := Validate(r)
	ret := RetentionOn(r, s.Normalised(), asOf)

	line("FORM F — ENTRY %d", r.Serial)
	line("Form for maintenance of record in case of pre-natal diagnostic test or")
	line("procedure by a genetic clinic, ultrasound clinic or imaging centre")
	line("[see proviso to s.4(3), rule 9(4) and rule 10(1A)] — Sections A, B and D")
	line("%s", strings.Repeat("=", 68))
	line("Item 1  Centre           : %s", orDash(c.Name))
	line("        Address          : %s", oneLine(orDash(c.Address)))
	line("Item 2  Registration no. : %s", orDash(c.RegistrationNo))
	line("")

	group := ""
	for _, l := range FormFace(r) {
		if l.Group != group {
			group = l.Group
			line("")
			line("%s", strings.ToUpper(group))
		}
		mark := " "
		if l.Missing {
			mark = "!"
		}
		item := l.Item
		if item == "" {
			item = "—"
		}
		line(" %s %-22s %-44s %s", mark, clip(item, 22), clip(l.Label, 44), orDash(oneLine(l.Value)))
	}
	line("")

	line("COMPLETENESS")
	line("  %s", v.Summary)
	for _, iss := range v.Issues {
		line("  ! %s", wrapIndent(iss.Message, 68, "    "))
	}
	line("")
	line("RETENTION")
	line("  %s", wrapIndent(ret.Note, 68, "  "))
	line("")
	line("  This page is an index to the signed Form F, the declarations, the referral")
	line("  slip and the case papers held at the centre. It is not a substitute for them.")
	return b.String()
}

// RenderReturnText renders the month's report as plain text: the covering
// summary first, then the entries it is assembled from, then the reasons it
// cannot be sent yet if there are any.
func RenderReturnText(ret MonthlyReturn) string {
	var b strings.Builder
	line := func(f string, a ...any) { fmt.Fprintf(&b, f+"\n", a...) }

	line("MONTHLY REPORT OF PROCEDURES — %s", strings.ToUpper(ret.MonthLabel))
	line("%s", strings.Repeat("=", 68))
	line("To              : %s", orDash(ret.Clinic.AppropriateAuthority))
	if ret.Clinic.District != "" {
		line("District        : %s", ret.Clinic.District)
	}
	line("From            : %s", orDash(ret.Clinic.Name))
	line("Registration no.: %s", orDash(ret.Clinic.RegistrationNo))
	if ret.Clinic.Address != "" {
		line("Address         : %s", oneLine(ret.Clinic.Address))
	}
	line("Month           : %s", ret.MonthLabel)
	line("Due by          : %s — day %d of the following month", ret.DueOn, ret.DueDay)
	if ret.Overdue {
		line("Status          : OVERDUE by %d day(s)", -ret.DaysToDue)
	} else {
		line("Status          : %d day(s) to the due date", ret.DaysToDue)
	}
	line("Prepared        : %s", ret.PreparedOn)
	line("")
	line("Rule            : %s", wrapIndent(ret.Reference, 68, "                  "))
	line("Format          : %s", wrapIndent(ret.FormatNote, 68, "                  "))
	line("")

	line("SUMMARY")
	line("  Form F entries for the month   : %d", ret.Totals.Entries)
	line("  Procedures counted by the centre: %d", ret.Check.Tallied)
	if ret.Totals.Entries > 0 {
		line("  Serial range                   : %d to %d", ret.Totals.SerialFirst, ret.Totals.SerialLast)
		line("  Dates                          : %s to %s across %d day(s)", ret.Totals.FirstDate, ret.Totals.LastDate, ret.Totals.DistinctDays)
		line("  Ultrasound / other procedures  : %d / %d", ret.Totals.Ultrasound, ret.Totals.OtherProcedures)
		line("  Self-referrals                 : %d", ret.Totals.SelfReferrals)
		line("  Entries noting an indication for a termination: %d", ret.Totals.MTPIndications)
		line("  Declarations given by thumb impression: %d", ret.Totals.ThumbImpression)
	}
	line("")

	for _, sec := range []struct {
		head   string
		counts []Count
	}{
		{"BY PROCEDURE", ret.Totals.Procedures},
		{"BY INDICATION — ITEM 10", ret.Totals.Indications},
		{"BY REFERRAL — ITEM 7", ret.Totals.Referrals},
	} {
		if len(sec.counts) == 0 {
			continue
		}
		line("%s", sec.head)
		for _, c := range sec.counts {
			line("  %4d  %s", c.Value, clip(c.Name, 62))
		}
		line("")
	}

	line("COMPLETENESS OF THE MONTH")
	if ret.Check.Clear {
		line("  Every counted procedure has a complete Form F and the numbering is intact.")
	} else {
		for _, f := range ret.Check.Findings {
			line("  ! %s", wrapIndent(f, 68, "    "))
		}
	}
	line("")

	if len(ret.Check.Days) > 0 {
		line("DAY BY DAY — FORM F AGAINST THE CENTRE'S OWN COUNT")
		w := tabwriter.NewWriter(&b, 0, 4, 2, ' ', 0)
		fmt.Fprintln(w, "  Date\tDay\tCounted\tForm F\tStatus\tSerials")
		for _, d := range ret.Check.Days {
			fmt.Fprintf(w, "  %s\t%s\t%d\t%d\t%s\t%s\n", d.Date, short3(d.Weekday), d.Tallied, d.Entries, d.Status, joinInts(d.Serials))
		}
		w.Flush()
		line("")
	}

	line("ENTRIES IN THIS REPORT")
	if len(ret.Entries) == 0 {
		line("  None.")
	} else {
		w := tabwriter.NewWriter(&b, 0, 4, 2, ' ', 0)
		fmt.Fprintln(w, "  Sl.\tDate\tName\tPerformed by\tIndication\tForm F")
		for _, r := range ret.Entries {
			state := "complete"
			if v := Validate(r); !v.Complete() {
				state = fmt.Sprintf("%d TO FIX", len(v.Issues))
			}
			fmt.Fprintf(w, "  %d\t%s\t%s\t%s\t%s\t%s\n", r.Serial, r.ProcedureDate,
				clip(r.PatientName, 22), clip(r.ConductedByName, 20),
				clip(joinIndications(r.Indications), 34), state)
		}
		w.Flush()
	}
	line("")

	if ret.Fileable {
		line("READY TO SEND")
		line("  Copy this onto the format your Appropriate Authority requires.")
	} else {
		line("NOT READY TO SEND")
		for _, blk := range ret.Blockers {
			line("  ! %s", wrapIndent(blk, 68, "    "))
		}
	}
	line("")
	line("AUTHENTICATION")
	auth := authenticationFor(ret.Clinic, ret.Settings, mustDay(ret.PreparedOn), "Monthly report under rule 9(8)")
	line("  %s", wrapIndent(auth.Statement, 68, "  "))
	line("")
	line("  %s", auth.SignatureL)
	line("")
	line("This tool files nothing. It submits nothing, sends nothing and connects to nothing.")
	line("The report has to be lodged with the Appropriate Authority in the manner that")
	line("office requires, and that office's requirements govern over anything stated here.")
	return b.String()
}

func mustDay(s string) time.Time {
	if d, err := ParseDay(s); err == nil {
		return d
	}
	return time.Time{}
}

func short3(s string) string {
	if len(s) <= 3 {
		return s
	}
	return s[:3]
}

func orDash(s string) string {
	if strings.TrimSpace(s) == "" {
		return "—"
	}
	return s
}

func oneLine(s string) string {
	return strings.Join(strings.Fields(strings.ReplaceAll(s, "\n", " ")), " ")
}

func clip(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	if n <= 1 {
		return string(r[:n])
	}
	return string(r[:n-1]) + "…"
}

// wrapIndent wraps text to width, indenting every line after the first.
func wrapIndent(s string, width int, indent string) string {
	words := strings.Fields(s)
	if len(words) == 0 {
		return ""
	}
	var b strings.Builder
	col := 0
	for i, w := range words {
		if i > 0 {
			if col+1+len(w) > width {
				b.WriteString("\n")
				b.WriteString(indent)
				col = len(indent)
			} else {
				b.WriteString(" ")
				col++
			}
		}
		b.WriteString(w)
		col += len(w)
	}
	return b.String()
}
