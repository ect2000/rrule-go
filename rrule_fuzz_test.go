//go:build go1.18
// +build go1.18

package rrule

import (
	"testing"
	"time"
)

// The decoder bounds work inside a single Next, independently of COUNT/UNTIL.
// Calendar frequencies start in January 9999: at most 365 daily periods, 53
// weekly periods, 12 months or one year remain, even for impossible selections.
// Subdaily frequencies start in the final minute of 9999, omit calendar filters
// and omit time filters at/above their frequency (the known unreachable-BY-list
// hang). Thus at most 60 seconds, one minute or one hour can be advanced. Only
// lower-order time expansions and reachable BYSETPOS +/-1 are used there.
// Each list has at most two entries: calendar timesets have at most 2^3 = 8
// entries, and the largest dayset has 366 entries. COUNT limits returned output,
// not search; the finite civil horizon and omitted filters bound search.
func fuzzIterateOption(freq, interval uint8, data []byte) ROption {
	maxInt := int(^uint(0) >> 1)
	intervals := []int{-1, 0, 1, 2, 7, 12, 24, 60, 366, maxInt - 1, maxInt, -maxInt - 1, 2147483647}
	option := ROption{
		Freq:     Frequency(freq % 7),
		Interval: intervals[int(interval)%len(intervals)],
		Dtstart:  time.Date(9999, 1, 1, 10, 0, 0, 0, time.UTC),
		Until:    time.Date(9999, 12, 31, 23, 59, 59, 0, time.UTC),
		Count:    3,
	}
	index := 0
	read := func() byte {
		index++
		if index > len(data) {
			return 0
		}
		return data[index-1]
	}
	// Always consume three bytes per field. A missing/zero length means no
	// selection. Boundary selectors also include invalid values; remaining
	// selectors explore signed byte values, without allocating from input size.
	list := func(boundaries []int) []int {
		n, a, b := int(read()%3), read(), read()
		var result []int
		for _, selector := range []byte{a, b}[:n] {
			value := int(int8(selector))
			if int(selector) < len(boundaries) {
				value = boundaries[int(selector)]
			}
			result = append(result, value)
		}
		return result
	}
	option.Bysetpos = list([]int{-367, -366, -2, -1, 0, 1, 2, 366, 367})
	option.Bymonth = list([]int{-1, 0, 1, 12, 13})
	option.Bymonthday = list([]int{-32, -31, -1, 0, 1, 31, 32})
	option.Byyearday = list([]int{-367, -366, -1, 0, 1, 366, 367})
	option.Byweekno = list([]int{-54, -53, -1, 0, 1, 53, 54})
	for i, ordinal := range list([]int{-54, -53, -8, -6, -5, -1, 0, 1, 5, 6, 8, 53, 54}) {
		weekday := []Weekday{MO, TU, WE, TH, FR, SA, SU}[(int(freq)+i)%7]
		option.Byweekday = append(option.Byweekday, weekday.Nth(ordinal))
	}
	option.Byhour = list([]int{-1, 0, 1, 10, 23, 24})
	option.Byminute = list([]int{-1, 0, 1, 30, 59, 60})
	option.Bysecond = list([]int{-1, 0, 1, 30, 59, 60})
	option.Byeaster = list([]int{-maxInt - 1, maxInt, -100, -81, -80, -1, 0, 1, 257, 258, 300})
	if option.Freq >= HOURLY {
		option.Dtstart = time.Date(9999, 12, 31, 23, 59, 0, 0, time.UTC)
		option.Bymonth, option.Bymonthday, option.Byyearday = nil, nil, nil
		option.Byweekno, option.Byweekday, option.Byeaster = nil, nil, nil
		option.Byhour = nil
		if option.Freq >= MINUTELY {
			option.Byminute = nil
		}
		if option.Freq == SECONDLY {
			option.Bysecond = nil
		}
		if len(option.Bysetpos) > 0 {
			option.Bysetpos = []int{1}
			if option.Interval%2 == 0 {
				option.Bysetpos[0] = -1
			}
		}
	}
	return option
}

// Keep both public paths as named subtests, so each deterministic seed can be
// replayed independently against unfixed production without recovering a panic.
func fuzzIteratePaths(t *testing.T, option ROption, requireFirst bool) {
	t.Helper()
	for _, path := range []string{"options", "text"} {
		t.Run(path, func(t *testing.T) {
			var rule *RRule
			var err error
			if path == "options" {
				rule, err = NewRRule(option)
			} else {
				rule, err = StrToRRule(option.String())
			}
			if err != nil {
				if requireFirst {
					t.Fatal(err)
				}
				return // Invalid boundaries are expected constructor refusals.
			}
			next := rule.Iterator()
			previous := option.Dtstart
			for i := 0; i <= option.Count; i++ {
				got, ok := next()
				if i == 0 && requireFirst && (!ok || !got.Equal(option.Dtstart)) {
					t.Fatalf("positive control: got %v, %v; want %v", got, ok, option.Dtstart)
				}
				if !ok {
					return
				}
				if i == option.Count || got.Before(previous) || got.Year() > MAXYEAR || got.After(option.Until) {
					t.Fatalf("invalid occurrence %d: %v (previous %v), rule %s", i, got, previous, option.String())
				}
				previous = got
			}
		})
	}
}

func TestFuzzIterateControls(t *testing.T) {
	// Same decoder, horizon and constructor/iterator harness as fuzzing. These
	// exact nonempty controls prevent rejection or an idle iterator passing.
	for freq := uint8(0); freq < 7; freq++ {
		option := fuzzIterateOption(freq, 2, nil)
		t.Run(option.Freq.String(), func(t *testing.T) {
			fuzzIteratePaths(t, option, true)
		})
	}
}

func FuzzIterate(f *testing.F) {
	seed := func(field, selector byte) []byte {
		data := make([]byte, 30)
		data[int(field)*3], data[int(field)*3+1] = 1, selector
		return data
	}
	// Each original panic family is isolated, without another filter hiding it.
	f.Add(uint8(MONTHLY), uint8(2), seed(5, 3)) // BYDAY=-6, January 9999.
	f.Add(uint8(YEARLY), uint8(2), seed(9, 10)) // BYEASTER=300, mask overflow.
	f.Add(uint8(MONTHLY), uint8(10), []byte{})  // Native MaxInt interval.
	for freq := uint8(0); freq < 7; freq++ {
		f.Add(freq, uint8(2), []byte{}) // Nonempty ordinary control.
		for interval := uint8(0); interval < 13; interval++ {
			f.Add(freq, interval, []byte{})
		}
	}
	// Commit every explicit field boundary, not only randomly discovered cases.
	for field, boundaries := range []int{9, 5, 7, 7, 7, 13, 6, 6, 6, 11} {
		for selector := 0; selector < boundaries; selector++ {
			f.Add(uint8(MONTHLY), uint8(2), seed(byte(field), byte(selector)))
		}
	}
	f.Fuzz(func(t *testing.T, freq, interval uint8, data []byte) {
		fuzzIteratePaths(t, fuzzIterateOption(freq, interval, data), false)
	})
}
