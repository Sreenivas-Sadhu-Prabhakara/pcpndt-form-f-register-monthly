package formf

import (
	"testing"
	"time"
)

func TestParseDay(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string // "" means an error is expected
	}{
		{"plain date", "2026-11-24", "2026-11-24"},
		{"leading and trailing space", "  2026-11-24 ", "2026-11-24"},
		{"leap day in a leap year", "2024-02-29", "2024-02-29"},
		{"first day of a century", "2000-01-01", "2000-01-01"},
		{"empty", "", ""},
		{"only spaces", "   ", ""},
		{"leap day in a common year", "2023-02-29", ""},
		{"thirty-first of February", "2026-02-31", ""},
		{"month thirteen", "2026-13-01", ""},
		{"day zero", "2026-11-00", ""},
		{"Indian order", "24-11-2026", ""},
		{"slashes", "2026/11/24", ""},
		{"with a clock", "2026-11-24T10:00:00Z", ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := ParseDay(c.in)
			if c.want == "" {
				if err == nil {
					t.Fatalf("ParseDay(%q) = %s, want an error", c.in, FormatDay(got))
				}
				return
			}
			if err != nil {
				t.Fatalf("ParseDay(%q) returned error %v, want %s", c.in, err, c.want)
			}
			if FormatDay(got) != c.want {
				t.Errorf("ParseDay(%q) = %s, want %s", c.in, FormatDay(got), c.want)
			}
			if h, m, s := got.Clock(); h != 0 || m != 0 || s != 0 {
				t.Errorf("ParseDay(%q) carries a clock %02d:%02d:%02d, want midnight", c.in, h, m, s)
			}
			if got.Location() != time.UTC {
				t.Errorf("ParseDay(%q) is in %v, want UTC so that day subtraction cannot drift", c.in, got.Location())
			}
		})
	}
}

func TestAddYearsClamped(t *testing.T) {
	cases := []struct {
		name  string
		from  string
		years int
		want  string
	}{
		{"leap day plus two years clamps into February", "2024-02-29", 2, "2026-02-28"},
		{"leap day plus four years stays a leap day", "2024-02-29", 4, "2028-02-29"},
		{"leap day plus one year clamps", "2024-02-29", 1, "2025-02-28"},
		{"28 February in a common year plus two", "2023-02-28", 2, "2025-02-28"},
		{"31 December plus two", "2024-12-31", 2, "2026-12-31"},
		{"31 March plus two", "2024-03-31", 2, "2026-03-31"},
		{"zero years is the same day", "2026-11-24", 0, "2026-11-24"},
		{"negative years go back", "2026-11-24", -2, "2024-11-24"},
		{"leap day backwards clamps too", "2024-02-29", -1, "2023-02-28"},
		{"a century that is not a leap year", "2000-02-29", 100, "2100-02-28"},
		{"a century that is a leap year", "1996-02-29", 4, "2000-02-29"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			from, err := ParseDay(c.from)
			if err != nil {
				t.Fatalf("test fixture %q does not parse: %v", c.from, err)
			}
			got := FormatDay(AddYearsClamped(from, c.years))
			if got != c.want {
				t.Errorf("AddYearsClamped(%s, %d) = %s, want %s", c.from, c.years, got, c.want)
			}
		})
	}
}

// TestAddYearsClampedNeverLeavesTheMonth pins the whole point of clamping: the
// retention deadline of a leap-day record must not slide into March.
func TestAddYearsClampedNeverLeavesTheMonth(t *testing.T) {
	from := Day(2024, time.February, 29)
	for n := -8; n <= 8; n++ {
		got := AddYearsClamped(from, n)
		if got.Month() != time.February {
			t.Errorf("AddYearsClamped(2024-02-29, %d) = %s, which is not in February", n, FormatDay(got))
		}
		if got.Year() != 2024+n {
			t.Errorf("AddYearsClamped(2024-02-29, %d) landed in %d, want %d", n, got.Year(), 2024+n)
		}
	}
}

func TestAddMonthsClamped(t *testing.T) {
	cases := []struct {
		name   string
		from   string
		months int
		want   string
	}{
		{"31 January plus one clamps to 28 February", "2026-01-31", 1, "2026-02-28"},
		{"31 January plus one in a leap year clamps to 29", "2024-01-31", 1, "2024-02-29"},
		{"31 January plus two is 31 March", "2026-01-31", 2, "2026-03-31"},
		{"31 May plus one clamps to 30 June", "2026-05-31", 1, "2026-06-30"},
		{"mid-month crossing into a new year", "2026-12-15", 1, "2027-01-15"},
		{"thirteen months on", "2026-11-24", 13, "2027-12-24"},
		{"twenty-four months on", "2026-11-24", 24, "2028-11-24"},
		{"zero months", "2026-11-24", 0, "2026-11-24"},
		{"one month back across a year boundary", "2026-01-15", -1, "2025-12-15"},
		{"thirteen months back", "2026-01-15", -13, "2024-12-15"},
		{"31 March back one clamps to 28 February", "2026-03-31", -1, "2026-02-28"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			from, err := ParseDay(c.from)
			if err != nil {
				t.Fatalf("test fixture %q does not parse: %v", c.from, err)
			}
			got := FormatDay(AddMonthsClamped(from, c.months))
			if got != c.want {
				t.Errorf("AddMonthsClamped(%s, %d) = %s, want %s", c.from, c.months, got, c.want)
			}
		})
	}
}

func TestDaysBetween(t *testing.T) {
	cases := []struct {
		name string
		a, b string
		want int
	}{
		{"same day", "2026-11-24", "2026-11-24", 0},
		{"one day", "2026-11-24", "2026-11-25", 1},
		{"backwards", "2026-11-25", "2026-11-24", -1},
		{"across February in a leap year", "2024-02-28", "2024-03-01", 2},
		{"across February in a common year", "2023-02-28", "2023-03-01", 1},
		{"a full leap year", "2024-01-01", "2025-01-01", 366},
		{"a full common year", "2023-01-01", "2024-01-01", 365},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			a, err := ParseDay(c.a)
			if err != nil {
				t.Fatalf("fixture %q: %v", c.a, err)
			}
			b, err := ParseDay(c.b)
			if err != nil {
				t.Fatalf("fixture %q: %v", c.b, err)
			}
			if got := DaysBetween(a, b); got != c.want {
				t.Errorf("DaysBetween(%s, %s) = %d, want %d", c.a, c.b, got, c.want)
			}
		})
	}
}

func TestMonth(t *testing.T) {
	cases := []struct {
		name                     string
		in                       string
		wantErr                  bool
		wantString, wantLabel    string
		wantFirst, wantLast      string
		wantDays                 int
		wantNext                 string
		inside, outside          []string
		wantValid                bool
		wantYear, wantMonthIndex int
	}{
		{
			name: "February in a leap year", in: "2024-02",
			wantString: "2024-02", wantLabel: "February 2024",
			wantFirst: "2024-02-01", wantLast: "2024-02-29", wantDays: 29,
			wantNext:  "2024-03",
			inside:    []string{"2024-02-01", "2024-02-29", "2024-02-15"},
			outside:   []string{"2024-01-31", "2024-03-01"},
			wantValid: true, wantYear: 2024, wantMonthIndex: 2,
		},
		{
			name: "February in a common year", in: "2026-02",
			wantString: "2026-02", wantLabel: "February 2026",
			wantFirst: "2026-02-01", wantLast: "2026-02-28", wantDays: 28,
			wantNext:  "2026-03",
			inside:    []string{"2026-02-28"},
			outside:   []string{"2026-02-29 is not a date", "2026-03-01"},
			wantValid: true, wantYear: 2026, wantMonthIndex: 2,
		},
		{
			name: "December rolls into the next year", in: "2026-12",
			wantString: "2026-12", wantLabel: "December 2026",
			wantFirst: "2026-12-01", wantLast: "2026-12-31", wantDays: 31,
			wantNext:  "2027-01",
			inside:    []string{"2026-12-31"},
			outside:   []string{"2027-01-01"},
			wantValid: true, wantYear: 2026, wantMonthIndex: 12,
		},
		{
			name: "a thirty-day month", in: "2026-11",
			wantString: "2026-11", wantLabel: "November 2026",
			wantFirst: "2026-11-01", wantLast: "2026-11-30", wantDays: 30,
			wantNext:  "2026-12",
			inside:    []string{"2026-11-30"},
			outside:   []string{"2026-12-01"},
			wantValid: true, wantYear: 2026, wantMonthIndex: 11,
		},
		{name: "month thirteen", in: "2026-13", wantErr: true},
		{name: "month zero", in: "2026-00", wantErr: true},
		{name: "a full date is not a month", in: "2026-11-24", wantErr: true},
		{name: "empty", in: "", wantErr: true},
		{name: "words", in: "November 2026", wantErr: true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			m, err := ParseMonth(c.in)
			if c.wantErr {
				if err == nil {
					t.Fatalf("ParseMonth(%q) = %v, want an error", c.in, m)
				}
				return
			}
			if err != nil {
				t.Fatalf("ParseMonth(%q): %v", c.in, err)
			}
			if m.String() != c.wantString {
				t.Errorf("String() = %q, want %q", m.String(), c.wantString)
			}
			if m.Label() != c.wantLabel {
				t.Errorf("Label() = %q, want %q", m.Label(), c.wantLabel)
			}
			if got := FormatDay(m.First()); got != c.wantFirst {
				t.Errorf("First() = %s, want %s", got, c.wantFirst)
			}
			if got := FormatDay(m.Last()); got != c.wantLast {
				t.Errorf("Last() = %s, want %s", got, c.wantLast)
			}
			if m.Days() != c.wantDays {
				t.Errorf("Days() = %d, want %d", m.Days(), c.wantDays)
			}
			if m.Next().String() != c.wantNext {
				t.Errorf("Next() = %s, want %s", m.Next(), c.wantNext)
			}
			if !m.Valid() {
				t.Errorf("Valid() = false for %s", m)
			}
			if m.Year != c.wantYear || int(m.Mon) != c.wantMonthIndex {
				t.Errorf("parsed as %d-%d, want %d-%d", m.Year, int(m.Mon), c.wantYear, c.wantMonthIndex)
			}
			if got := MonthOf(m.Last()); got != m {
				t.Errorf("MonthOf(%s) = %s, want %s", FormatDay(m.Last()), got, m)
			}
			for _, s := range c.inside {
				d, err := ParseDay(s)
				if err != nil {
					continue
				}
				if !m.Contains(d) {
					t.Errorf("%s.Contains(%s) = false, want true", m, s)
				}
			}
			for _, s := range c.outside {
				d, err := ParseDay(s)
				if err != nil {
					continue
				}
				if m.Contains(d) {
					t.Errorf("%s.Contains(%s) = true, want false", m, s)
				}
			}
		})
	}
}

func TestMonthZeroValueIsNotValid(t *testing.T) {
	var m Month
	if m.Valid() {
		t.Error("the zero Month reports itself valid; a register must not report on year zero")
	}
}
