package formf

import (
	"fmt"
	"strings"
	"time"
)

// Calendar arithmetic for the register.
//
// Every date in a Form F record travels as a plain "YYYY-MM-DD" string: a
// register entry is about a day, not an instant, and a day has no timezone. A
// scan done on 1 April in Nagpur was done on 1 April, whatever the clock on
// the machine the register runs on says. Parsing to a UTC midnight keeps that
// true and keeps subtraction honest.
//
// The one piece of real arithmetic here is adding calendar years for the
// retention window, and the interesting case is a record made on a leap day.
// 29 February 2024 plus two years is not a date. Go's own AddDate rolls the
// overflow forward and gives 1 March 2026; a register must not quietly move a
// retention deadline into a later month, so these helpers clamp to the last
// day of the target month instead.

// dayLayout is the one date format the register reads and writes.
const dayLayout = "2006-01-02"

// monthLayout is the one month format the register reads and writes.
const monthLayout = "2006-01"

// ParseDay reads a "YYYY-MM-DD" calendar date. It is strict: an impossible
// date such as 2026-02-30 is an error, not a rolled-forward 2 March.
func ParseDay(s string) (time.Time, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return time.Time{}, fmt.Errorf("empty date")
	}
	t, err := time.ParseInLocation(dayLayout, s, time.UTC)
	if err != nil {
		return time.Time{}, fmt.Errorf("date %q is not a calendar date in YYYY-MM-DD form", s)
	}
	return t, nil
}

// FormatDay renders a date back to "YYYY-MM-DD".
func FormatDay(t time.Time) string { return t.UTC().Format(dayLayout) }

// Day builds a calendar date directly.
func Day(year int, month time.Month, day int) time.Time {
	return time.Date(year, month, day, 0, 0, 0, 0, time.UTC)
}

// dayOf drops the clock and the zone from an instant, leaving the calendar day
// it fell on. Every comparison in the register is between days.
func dayOf(t time.Time) time.Time {
	t = t.UTC()
	return Day(t.Year(), t.Month(), t.Day())
}

// daysInMonth returns the length of the given month, leap years included.
func daysInMonth(year int, month time.Month) int {
	// Day zero of the next month is the last day of this one.
	return time.Date(year, month+1, 0, 0, 0, 0, 0, time.UTC).Day()
}

// AddYearsClamped advances t by n calendar years, clamping the day of the
// month to the last day of the target month rather than overflowing into the
// next one. 29 February 2024 plus 2 becomes 28 February 2026, not 1 March.
func AddYearsClamped(t time.Time, n int) time.Time {
	t = t.UTC()
	return clamp(t.Year()+n, t.Month(), t.Day())
}

// AddMonthsClamped advances t by n calendar months with the same clamping
// rule. 31 January plus 1 becomes 28 or 29 February, not 2 or 3 March.
func AddMonthsClamped(t time.Time, n int) time.Time {
	t = t.UTC()
	total := int(t.Month()) - 1 + n
	year := t.Year() + floorDiv(total, 12)
	month := time.Month(floorMod(total, 12) + 1)
	return clamp(year, month, t.Day())
}

func clamp(year int, month time.Month, day int) time.Time {
	if max := daysInMonth(year, month); day > max {
		day = max
	}
	return Day(year, month, day)
}

func floorDiv(a, b int) int {
	q := a / b
	if a%b != 0 && (a < 0) != (b < 0) {
		q--
	}
	return q
}

func floorMod(a, b int) int { return a - floorDiv(a, b)*b }

// DaysBetween returns the number of whole days from a to b, negative if b is
// the earlier date.
func DaysBetween(a, b time.Time) int {
	return int(b.UTC().Sub(a.UTC()).Hours() / 24)
}

// Month is a calendar month: the cycle the register reports on.
type Month struct {
	Year int
	Mon  time.Month
}

// ParseMonth reads a "YYYY-MM" month.
func ParseMonth(s string) (Month, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return Month{}, fmt.Errorf("empty month")
	}
	t, err := time.ParseInLocation(monthLayout, s, time.UTC)
	if err != nil {
		return Month{}, fmt.Errorf("month %q is not a month in YYYY-MM form", s)
	}
	return Month{Year: t.Year(), Mon: t.Month()}, nil
}

// MonthOf returns the month a date falls in.
func MonthOf(t time.Time) Month {
	t = t.UTC()
	return Month{Year: t.Year(), Mon: t.Month()}
}

// String renders the month as "YYYY-MM".
func (m Month) String() string { return m.First().Format(monthLayout) }

// Label renders the month for a human, as "November 2026".
func (m Month) Label() string { return m.First().Format("January 2006") }

// First returns the first day of the month.
func (m Month) First() time.Time { return Day(m.Year, m.Mon, 1) }

// Last returns the last day of the month.
func (m Month) Last() time.Time { return Day(m.Year, m.Mon, daysInMonth(m.Year, m.Mon)) }

// Days returns the number of days in the month.
func (m Month) Days() int { return daysInMonth(m.Year, m.Mon) }

// Next returns the following month.
func (m Month) Next() Month { return MonthOf(AddMonthsClamped(m.First(), 1)) }

// Contains reports whether a date falls inside the month.
func (m Month) Contains(t time.Time) bool {
	t = t.UTC()
	return !t.Before(m.First()) && !t.After(m.Last())
}

// Valid reports whether the month is a real month rather than a zero value.
func (m Month) Valid() bool { return m.Year > 0 && m.Mon >= time.January && m.Mon <= time.December }
