package formf

import (
	"reflect"
	"strings"
	"testing"
)

// entries builds a register from serial/date pairs, with nothing else filled
// in, because the numbering checks look at nothing else.
func entries(pairs ...any) []Record {
	var out []Record
	for i := 0; i+1 < len(pairs); i += 2 {
		out = append(out, Record{Serial: pairs[i].(int), ProcedureDate: pairs[i+1].(string)})
	}
	return out
}

func TestCheckSerials(t *testing.T) {
	cases := []struct {
		name          string
		records       []Record
		openingSerial int
		wantIntact    bool
		wantDisorder  bool
		wantProblems  []SerialProblem
		wantMissing   int
		wantFirstLast [2]int
		// wantIn checks that a particular number or phrase reaches the
		// message, because these messages are what the clinic acts on.
		wantIn []string
	}{
		{
			name:       "an empty register is intact",
			records:    nil,
			wantIntact: true,
		},
		{
			name:          "one entry at the opening serial",
			records:       entries(1, "2026-11-01"),
			wantIntact:    true,
			wantFirstLast: [2]int{1, 1},
		},
		{
			name:          "an unbroken run",
			records:       entries(1, "2026-11-01", 2, "2026-11-02", 3, "2026-11-03"),
			wantIntact:    true,
			wantFirstLast: [2]int{1, 3},
		},
		{
			name:          "one number missing in the middle",
			records:       entries(1, "2026-11-01", 2, "2026-11-02", 4, "2026-11-04"),
			wantProblems:  []SerialProblem{SerialGap},
			wantMissing:   1,
			wantFirstLast: [2]int{1, 4},
			wantIn:        []string{"Serial 3 is missing", "reads as a removed record"},
		},
		{
			name:          "a run of numbers missing",
			records:       entries(1, "2026-11-01", 6, "2026-11-06"),
			wantProblems:  []SerialProblem{SerialGap},
			wantMissing:   4,
			wantFirstLast: [2]int{1, 6},
			wantIn:        []string{"Serials 2 to 5 are missing — 4 entries"},
		},
		{
			name:          "two separate gaps",
			records:       entries(1, "2026-11-01", 3, "2026-11-03", 6, "2026-11-06"),
			wantProblems:  []SerialProblem{SerialGap, SerialGap},
			wantMissing:   3,
			wantFirstLast: [2]int{1, 6},
			wantIn:        []string{"Serial 2 is missing", "Serials 4 to 5 are missing"},
		},
		{
			name:          "one number used twice",
			records:       entries(1, "2026-11-01", 2, "2026-11-02", 2, "2026-11-02"),
			wantProblems:  []SerialProblem{SerialDuplicate},
			wantFirstLast: [2]int{1, 2},
			wantIn:        []string{"Serial 2 is used by 2 entries"},
		},
		{
			name:          "a zero serial",
			records:       entries(0, "2026-11-01", 1, "2026-11-02"),
			wantProblems:  []SerialProblem{SerialNotPositive},
			wantFirstLast: [2]int{0, 1},
			wantIn:        []string{"Entry numbered 0"},
		},
		{
			name:          "a negative serial",
			records:       entries(-3, "2026-11-01", 1, "2026-11-02"),
			wantProblems:  []SerialProblem{SerialNotPositive},
			wantFirstLast: [2]int{-3, 1},
		},
		{
			name:          "a register that does not start where it says it does",
			records:       entries(5, "2026-11-05", 6, "2026-11-06"),
			wantProblems:  []SerialProblem{SerialOpeningMismatch},
			wantFirstLast: [2]int{5, 6},
			wantIn:        []string{"set to open at serial 1 but its first entry is 5", "4 entries are unaccounted for"},
		},
		{
			name:          "a register carried over from paper, opening at 412",
			records:       entries(412, "2026-11-05", 413, "2026-11-06"),
			openingSerial: 412,
			wantIntact:    true,
			wantFirstLast: [2]int{412, 413},
		},
		{
			name:          "an entry from the earlier paper register",
			records:       entries(410, "2026-10-29", 412, "2026-11-05"),
			openingSerial: 412,
			wantProblems:  []SerialProblem{SerialBelowOpening},
			wantFirstLast: [2]int{410, 412},
			wantIn:        []string{"before this register's opening serial of 412"},
		},
		{
			// The numbering itself is sound here: nothing is missing
			// and nothing is repeated. What is odd is the date, and
			// that is reported separately because a Form F written up
			// late is a correction rather than a defect.
			name:          "a later number carrying an earlier date",
			records:       entries(1, "2026-11-05", 2, "2026-11-02"),
			wantIntact:    true,
			wantDisorder:  true,
			wantProblems:  []SerialProblem{SerialDateOutOfOrder},
			wantFirstLast: [2]int{1, 2},
			wantIn:        []string{"the shape a back-dated entry takes", "say so in its remarks"},
		},
		{
			name:          "two entries on the same day are in order",
			records:       entries(1, "2026-11-05", 2, "2026-11-05"),
			wantIntact:    true,
			wantFirstLast: [2]int{1, 2},
		},
		{
			name:          "entries with no date are skipped by the date check",
			records:       entries(1, "", 2, "2026-11-02", 3, ""),
			wantIntact:    true,
			wantFirstLast: [2]int{1, 3},
		},
		{
			name:          "a gap and a back-dated entry at once",
			records:       entries(1, "2026-11-05", 3, "2026-11-02"),
			wantDisorder:  true,
			wantProblems:  []SerialProblem{SerialGap, SerialDateOutOfOrder},
			wantMissing:   1,
			wantFirstLast: [2]int{1, 3},
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			s := DefaultSettings()
			if c.openingSerial != 0 {
				s.OpeningSerial = c.openingSerial
			}
			got := CheckSerials(c.records, s)

			if got.Intact != c.wantIntact {
				t.Errorf("Intact = %v, want %v; issues: %v", got.Intact, c.wantIntact, got.Issues)
			}
			if got.Ordered != !c.wantDisorder {
				t.Errorf("Ordered = %v, want %v", got.Ordered, !c.wantDisorder)
			}
			if n := len(got.NumberingIssues()) + len(got.OrderIssues()); n != len(got.Issues) {
				t.Errorf("the two issue views hold %d between them against %d issues", n, len(got.Issues))
			}
			if got.Intact != (len(got.NumberingIssues()) == 0) {
				t.Error("Intact and NumberingIssues disagree")
			}
			var problems []SerialProblem
			for _, iss := range got.Issues {
				problems = append(problems, iss.Problem)
			}
			if !reflect.DeepEqual(problems, c.wantProblems) {
				t.Errorf("problems = %v, want %v", problems, c.wantProblems)
				for _, iss := range got.Issues {
					t.Logf("  %s: %s", iss.Problem, iss.Message)
				}
			}
			if got.Missing != c.wantMissing {
				t.Errorf("Missing = %d, want %d", got.Missing, c.wantMissing)
			}
			if len(c.records) > 0 && (got.First != c.wantFirstLast[0] || got.Last != c.wantFirstLast[1]) {
				t.Errorf("First, Last = %d, %d; want %d, %d", got.First, got.Last, c.wantFirstLast[0], c.wantFirstLast[1])
			}
			if got.Entries != len(c.records) {
				t.Errorf("Entries = %d, want %d", got.Entries, len(c.records))
			}
			all := ""
			for _, iss := range got.Issues {
				all += iss.Message + "\n"
				if iss.Message == "" {
					t.Errorf("issue %s carries no message", iss.Problem)
				}
			}
			for _, want := range c.wantIn {
				if !strings.Contains(all, want) {
					t.Errorf("no message contains %q; messages were:\n%s", want, all)
				}
			}
		})
	}
}

func TestCheckSerialsDoesNotReorderTheCaller(t *testing.T) {
	in := entries(3, "2026-11-03", 1, "2026-11-01", 2, "2026-11-02")
	before := []int{in[0].Serial, in[1].Serial, in[2].Serial}
	CheckSerials(in, DefaultSettings())
	after := []int{in[0].Serial, in[1].Serial, in[2].Serial}
	if !reflect.DeepEqual(before, after) {
		t.Errorf("CheckSerials reordered the caller's slice: %v became %v", before, after)
	}
}

func TestNextSerial(t *testing.T) {
	cases := []struct {
		name          string
		records       []Record
		openingSerial int
		want          int
	}{
		{"an empty register", nil, 0, 1},
		{"an empty register carried over from paper", nil, 412, 412},
		{"after three entries", entries(1, "", 2, "", 3, ""), 0, 4},
		{"out of order in the file", entries(3, "", 1, "", 2, ""), 0, 4},
		// A number left free by a gap is never reissued: two different
		// procedures must not end up sharing one identity in the file.
		{"a gap is not filled in", entries(1, "", 2, "", 7, ""), 0, 8},
		{"a stray high number still advances the count", entries(1, "", 900, ""), 0, 901},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			s := DefaultSettings()
			if c.openingSerial != 0 {
				s.OpeningSerial = c.openingSerial
			}
			if got := NextSerial(c.records, s); got != c.want {
				t.Errorf("NextSerial = %d, want %d", got, c.want)
			}
		})
	}
}
