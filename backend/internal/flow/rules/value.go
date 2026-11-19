// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package rules

import (
	"cmp"
	"math"
	"strconv"
	"strings"
	"time"
)

// decimal is a decimal number in canonical form: integer digits without leading zeros and fraction
// digits without trailing zeros. Zero is never negative.
type decimal struct {
	negative bool
	integer  string
	fraction string
}

// parseDecimal reads an optional minus sign, one or more digits, and optionally a point followed by
// one or more digits. A plus sign, an exponent, spaces and a leading or trailing point are refused.
func parseDecimal(s string) (decimal, bool) {
	var d decimal
	i := 0
	if i < len(s) && s[i] == '-' {
		d.negative = true
		i++
	}
	start := i
	for i < len(s) && isDigit(s[i]) {
		i++
	}
	if i == start {
		return decimal{}, false
	}
	d.integer = strings.TrimLeft(s[start:i], "0")
	if d.integer == "" {
		d.integer = "0"
	}
	if i < len(s) && s[i] == '.' {
		i++
		fracStart := i
		for i < len(s) && isDigit(s[i]) {
			i++
		}
		if i == fracStart {
			return decimal{}, false
		}
		d.fraction = strings.TrimRight(s[fracStart:i], "0")
	}
	if i != len(s) {
		return decimal{}, false
	}
	if d.integer == "0" && d.fraction == "" {
		d.negative = false
	}
	return d, true
}

// String returns the canonical text of the number.
func (d decimal) String() string {
	var b strings.Builder
	if d.negative {
		b.WriteByte('-')
	}
	b.WriteString(d.integer)
	if d.fraction != "" {
		b.WriteByte('.')
		b.WriteString(d.fraction)
	}
	return b.String()
}

// compareDecimal returns -1, 0 or 1 as a is less than, equal to or greater than b.
func compareDecimal(a, b decimal) int {
	if a.negative != b.negative {
		if a.negative {
			return -1
		}
		return 1
	}
	c := compareMagnitude(a, b)
	if a.negative {
		return -c
	}
	return c
}

// compareMagnitude compares the absolute values of two canonical decimals.
func compareMagnitude(a, b decimal) int {
	if len(a.integer) != len(b.integer) {
		return cmp.Compare(len(a.integer), len(b.integer))
	}
	if c := strings.Compare(a.integer, b.integer); c != 0 {
		return c
	}
	for i := range max(len(a.fraction), len(b.fraction)) {
		da, db := digitAt(a.fraction, i), digitAt(b.fraction, i)
		if da != db {
			return cmp.Compare(da, db)
		}
	}
	return 0
}

// digitAt returns the digit at position i of s, or '0' past its end.
func digitAt(s string, i int) byte {
	if i < len(s) {
		return s[i]
	}
	return '0'
}

// isDigit reports whether c is an ASCII digit.
func isDigit(c byte) bool {
	return c >= '0' && c <= '9'
}

// parseInstant reads a YYYY-MM-DD date, taken as midnight UTC, or an RFC 3339 timestamp with a zone.
func parseInstant(s string) (time.Time, bool) {
	if len(s) == isoDateLength {
		t, err := time.Parse(time.DateOnly, s)
		return t, err == nil
	}
	if len(s) <= isoDateLength || s[isoDateLength] != 'T' {
		return time.Time{}, false
	}
	t, err := time.Parse(time.RFC3339Nano, s)
	return t, err == nil && validOffset(s)
}

// validOffset reports whether the zone of a parsed timestamp is Z or an offset with hours 00 to 23 and
// minutes 00 to 59. time.Parse accepts offsets such as +24:00 and +05:60, which RFC 3339 does not.
func validOffset(s string) bool {
	if s[len(s)-1] == 'Z' {
		return true
	}
	zone := s[len(s)-zoneOffsetLength:]
	return zone[1:3] <= maxOffsetHours && zone[4:6] <= maxOffsetMinutes
}

// isoDuration is an ISO 8601 duration. Years and months are kept apart because their length depends
// on the date they are applied to.
type isoDuration struct {
	years   int
	months  int
	days    int
	seconds int64
}

// parseDuration reads an ISO 8601 duration such as P30D, PT15M or P1Y2M3DT4H5M6S. Components are
// whole numbers of at most nine digits, in the standard order, and at least one is present.
func parseDuration(s string) (isoDuration, bool) {
	var d isoDuration
	if len(s) < 3 || s[0] != 'P' {
		return d, false
	}
	units := "YMWD"
	inTime := false
	seen := 0
	i := 1
	for i < len(s) {
		if s[i] == 'T' {
			if inTime || i == len(s)-1 {
				return d, false
			}
			inTime, units = true, "HMS"
			i++
			continue
		}
		start := i
		for i < len(s) && isDigit(s[i]) {
			i++
		}
		if i == start || i-start > maxDurationDigits || i == len(s) {
			return d, false
		}
		pos := strings.IndexByte(units, s[i])
		if pos < 0 {
			return d, false
		}
		n, err := strconv.Atoi(s[start:i])
		if err != nil {
			return d, false
		}
		d.add(inTime, s[i], n)
		units = units[pos+1:]
		seen++
		i++
	}
	return d, seen > 0
}

// add adds n of the given unit to the duration.
func (d *isoDuration) add(inTime bool, unit byte, n int) {
	if inTime {
		switch unit {
		case 'H':
			d.seconds += int64(n) * secondsPerHour
		case 'M':
			d.seconds += int64(n) * secondsPerMinute
		default:
			d.seconds += int64(n)
		}
		return
	}
	switch unit {
	case 'Y':
		d.years += n
	case 'M':
		d.months += n
	case 'W':
		d.days += n * daysPerWeek
	default:
		d.days += n
	}
}

// nominalSeconds is the length used to order durations: a year is 365 days and a month 30 days.
func (d isoDuration) nominalSeconds() int64 {
	days := int64(d.years)*nominalDaysPerYear + int64(d.months)*nominalDaysPerMonth + int64(d.days)
	return days*secondsPerDay + d.seconds
}

// before returns the instant the duration lies before now, using calendar arithmetic. Years and months
// keep the day of the month, or take the last day of a shorter month. Calendar steps are taken only for
// nonzero components, since rebuilding a repeated wall-clock time at a DST change can move it by an hour.
// The seconds are subtracted in steps a time.Duration can hold, since nine digits of hours exceed its
// range.
func (d isoDuration) before(now time.Time) time.Time {
	const maxStep = math.MaxInt64 / int64(time.Second)
	t := now
	if d.years != 0 || d.months != 0 {
		t = now.AddDate(-d.years, -d.months, 0)
		if t.Day() != now.Day() {
			t = t.AddDate(0, 0, -t.Day())
		}
	}
	if d.days != 0 {
		t = t.AddDate(0, 0, -d.days)
	}
	for rest := d.seconds; rest > 0; rest -= maxStep {
		t = t.Add(-time.Duration(min(rest, maxStep)) * time.Second)
	}
	return t
}

// atom is one value of a signal or literal, read once in every way it can be compared.
type atom struct {
	text    string
	lower   string
	classes comparatorMask
	number  decimal
	instant time.Time
	period  isoDuration
}

// newAtom reads text as every inferable comparator that recognizes it.
func newAtom(text string) atom {
	a := atom{text: text, lower: strings.ToLower(text), classes: maskOf(cmpText)}
	if n, ok := parseDecimal(text); ok {
		a.number = n
		a.classes |= maskOf(cmpNumber)
	}
	if t, ok := parseInstant(text); ok {
		a.instant = t
		a.classes |= maskOf(cmpInstant)
	}
	if d, ok := parseDuration(text); ok {
		a.period = d
		a.classes |= maskOf(cmpDuration)
	}
	if text == "true" || text == "false" {
		a.classes |= maskOf(cmpBoolean)
	}
	return a
}

// newAtoms reads every value of a list.
func newAtoms(values []string) []atom {
	atoms := make([]atom, len(values))
	for i, v := range values {
		atoms[i] = newAtom(v)
	}
	return atoms
}

// isBlank reports whether the atom has no non-whitespace character.
func (a *atom) isBlank() bool {
	return strings.TrimSpace(a.text) == ""
}
