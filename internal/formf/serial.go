package formf

import (
	"fmt"
	"sort"
)

// Serial integrity.
//
// A register is evidence, and what makes it evidence is that it cannot have
// had a page torn out of it. An inspector reading a bound register checks
// exactly this: that the entries run 1, 2, 3 with nothing missing, that no
// number appears twice, and that the dates march forward with the numbers. A
// gap at entry 118 is not a clerical slip, it is the shape of a record that
// was removed, and it is read that way.
//
// So this register assigns serial numbers itself, never accepts one typed by
// hand, and checks the whole sequence on every read. Problems are reported in
// full and in plain words rather than counted.

// SerialProblem names one thing wrong with the numbering.
type SerialProblem string

const (
	// SerialGap is a missing number, or a run of them.
	SerialGap SerialProblem = "gap"
	// SerialDuplicate is one number used twice.
	SerialDuplicate SerialProblem = "duplicate"
	// SerialBelowOpening is a number before the register's first.
	SerialBelowOpening SerialProblem = "below-opening"
	// SerialNotPositive is a zero or negative number.
	SerialNotPositive SerialProblem = "not-positive"
	// SerialDateOutOfOrder is a later entry carrying an earlier date, which
	// is the shape of an entry written up after the fact.
	SerialDateOutOfOrder SerialProblem = "date-out-of-order"
	// SerialOpeningMismatch is a register whose first entry is not the
	// opening serial it was set up with.
	SerialOpeningMismatch SerialProblem = "opening-mismatch"
)

// SerialIssue is one problem with the numbering, stated so that it can be read
// aloud to the person holding the paper file.
type SerialIssue struct {
	Problem SerialProblem `json:"problem"`
	// From and To bound a run of missing numbers; for every other problem
	// they are the single serial concerned.
	From    int    `json:"from"`
	To      int    `json:"to"`
	Count   int    `json:"count"`
	Message string `json:"message"`
}

// SerialCheck is the state of a register's numbering.
type SerialCheck struct {
	Entries       int `json:"entries"`
	OpeningSerial int `json:"openingSerial"`
	First         int `json:"first"`
	Last          int `json:"last"`
	// Expected is how many entries there would be if the sequence from
	// First to Last were unbroken.
	Expected int `json:"expected"`
	Missing  int `json:"missing"`
	// Intact is true when the numbering itself is sound: no gap, no
	// number used twice, nothing outside the register's own range.
	Intact bool `json:"intact"`
	// Ordered is true when the dates march forward with the numbers.
	//
	// It is kept apart from Intact on purpose. A gap is a defect: a number
	// is missing and something has to account for it. An entry whose date
	// runs backwards against its number is not necessarily a defect — a
	// Form F found to be missing and written up a week later lands at the
	// end of the register with an earlier date, and writing it is the right
	// thing to have done. It is the shape a back-dated entry takes, so the
	// register says so and asks for it to be explained, and it does not
	// hold up the month's report the way a gap does.
	Ordered bool          `json:"ordered"`
	Issues  []SerialIssue `json:"issues"`
}

// NumberingIssues returns the problems with the numbering itself, leaving out
// the dates-against-numbers findings.
func (c SerialCheck) NumberingIssues() []SerialIssue {
	var out []SerialIssue
	for _, i := range c.Issues {
		if i.Problem != SerialDateOutOfOrder {
			out = append(out, i)
		}
	}
	return out
}

// OrderIssues returns the dates-against-numbers findings alone.
func (c SerialCheck) OrderIssues() []SerialIssue {
	var out []SerialIssue
	for _, i := range c.Issues {
		if i.Problem == SerialDateOutOfOrder {
			out = append(out, i)
		}
	}
	return out
}

// NextSerial returns the number the next entry should take: one past the
// highest in use, or the opening serial for an empty register. It never
// reissues a number, even one left free by a gap, because reusing a number
// would make two different procedures share an identity in the file.
func NextSerial(records []Record, s Settings) int {
	next := s.Normalised().OpeningSerial
	for _, r := range records {
		if r.Serial >= next {
			next = r.Serial + 1
		}
	}
	return next
}

// CheckSerials reports everything wrong with a register's numbering.
func CheckSerials(records []Record, s Settings) SerialCheck {
	s = s.Normalised()
	out := SerialCheck{
		Entries:       len(records),
		OpeningSerial: s.OpeningSerial,
		Intact:        true,
		Ordered:       true,
	}
	if len(records) == 0 {
		return out
	}

	// Work on a serial-ordered copy; the caller's slice is left alone.
	ordered := make([]Record, len(records))
	copy(ordered, records)
	sort.SliceStable(ordered, func(i, j int) bool { return ordered[i].Serial < ordered[j].Serial })

	seen := make(map[int]int, len(ordered))
	for _, r := range ordered {
		seen[r.Serial]++
	}

	out.First = ordered[0].Serial
	out.Last = ordered[len(ordered)-1].Serial

	// Numbers that cannot be serials at all.
	for _, n := range sortedKeys(seen) {
		if n <= 0 {
			out.Issues = append(out.Issues, SerialIssue{
				Problem: SerialNotPositive, From: n, To: n, Count: seen[n],
				Message: fmt.Sprintf("Entry numbered %d. A register serial starts at %d and counts up; this entry has no place in the sequence.", n, s.OpeningSerial),
			})
			continue
		}
		if n < s.OpeningSerial {
			out.Issues = append(out.Issues, SerialIssue{
				Problem: SerialBelowOpening, From: n, To: n, Count: seen[n],
				Message: fmt.Sprintf("Entry %d sits before this register's opening serial of %d. Either the opening serial is set wrong or this entry belongs in the earlier register.", n, s.OpeningSerial),
			})
		}
	}

	// One number used twice.
	for _, n := range sortedKeys(seen) {
		if seen[n] > 1 {
			out.Issues = append(out.Issues, SerialIssue{
				Problem: SerialDuplicate, From: n, To: n, Count: seen[n],
				Message: fmt.Sprintf("Serial %d is used by %d entries. Two procedures cannot share one Form F number — renumber one of them to the next free serial and correct the paper file to match.", n, seen[n]),
			})
		}
	}

	// A register that does not begin where it says it begins.
	if out.First != s.OpeningSerial {
		if out.First > s.OpeningSerial {
			out.Issues = append(out.Issues, SerialIssue{
				Problem: SerialOpeningMismatch, From: s.OpeningSerial, To: out.First - 1,
				Count:   out.First - s.OpeningSerial,
				Message: fmt.Sprintf("This register is set to open at serial %d but its first entry is %d. %d entries are unaccounted for at the front of the register.", s.OpeningSerial, out.First, out.First-s.OpeningSerial),
			})
		}
	}

	// Gaps, reported as runs rather than one line per number.
	from := max(out.First, s.OpeningSerial)
	n := from
	for n <= out.Last {
		if seen[n] > 0 {
			n++
			continue
		}
		start := n
		for n <= out.Last && seen[n] == 0 {
			n++
		}
		end := n - 1
		count := end - start + 1
		out.Missing += count
		msg := fmt.Sprintf("Serial %d is missing.", start)
		if count > 1 {
			msg = fmt.Sprintf("Serials %d to %d are missing — %d entries.", start, end, count)
		}
		out.Issues = append(out.Issues, SerialIssue{
			Problem: SerialGap, From: start, To: end, Count: count,
			Message: msg + " A gap in a register reads as a removed record. Find the Form F for each missing number and enter it, or record in writing why the number was never used.",
		})
	}

	// Dates that run backwards against the numbering.
	var prevSerial int
	var prevDate string
	for _, r := range ordered {
		if r.ProcedureDate == "" {
			continue
		}
		d, err := ParseDay(r.ProcedureDate)
		if err != nil {
			continue
		}
		if prevDate != "" {
			p, err := ParseDay(prevDate)
			if err == nil && d.Before(p) {
				out.Issues = append(out.Issues, SerialIssue{
					Problem: SerialDateOutOfOrder, From: r.Serial, To: r.Serial, Count: 1,
					Message: fmt.Sprintf("Entry %d is dated %s but the earlier entry %d is dated %s. A later number carrying an earlier date is the shape a back-dated entry takes. If this is a Form F that was found missing and written up afterwards, say so in its remarks — that is a correction, and it is worth being able to point to. If it is not, one of the two dates is wrong.", r.Serial, r.ProcedureDate, prevSerial, prevDate),
				})
			}
		}
		prevSerial, prevDate = r.Serial, r.ProcedureDate
	}

	if out.Last >= out.First {
		out.Expected = out.Last - out.First + 1
	}
	out.Intact = len(out.NumberingIssues()) == 0
	out.Ordered = len(out.OrderIssues()) == 0
	return out
}

func sortedKeys(m map[int]int) []int {
	out := make([]int, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Ints(out)
	return out
}
