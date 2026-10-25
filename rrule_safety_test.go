package rrule

import (
	"fmt"
	"strconv"
	"testing"
	"time"
)

func safetyDate(s string) time.Time {
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		panic(err)
	}
	return t
}

// Exercise both public constructors and include Iterator creation in the check:
// mask construction can panic before the first Next call.
func safetyOccurrences(t *testing.T, option ROption, want ...string) {
	t.Helper()
	for _, path := range []string{"options", "text"} {
		t.Run(path, func(t *testing.T) {
			var r *RRule
			var err error
			if path == "options" {
				r, err = NewRRule(option)
			} else {
				r, err = StrToRRule(option.String())
			}
			if err != nil {
				t.Fatal(err)
			}
			next := r.Iterator()
			for i, expected := range want {
				got, ok := next()
				if !ok || got.Format(time.RFC3339) != expected {
					t.Fatalf("occurrence %d: got %v, %v; want %s", i, got, ok, expected)
				}
			}
			if got, ok := next(); ok {
				t.Fatalf("unexpected occurrence after %d: %v", len(want), got)
			}
		})
	}
}

func TestSafetyOrdinal(t *testing.T) {
	start := safetyDate("2026-01-05T10:00:00Z")
	for _, tc := range []struct {
		name  string
		freq  Frequency
		month []int
		days  []Weekday
		want  []string
	}{
		{"monthly_negative6", MONTHLY, nil, []Weekday{MO.Nth(-6)}, nil},
		{"monthly_8", MONTHLY, nil, []Weekday{MO.Nth(8)}, nil},
		{"monthly_53", MONTHLY, nil, []Weekday{MO.Nth(53)}, nil},
		{"yearly_jan_negative6", YEARLY, []int{1}, []Weekday{MO.Nth(-6)}, nil},
		{"yearly_dec_8", YEARLY, []int{12}, []Weekday{MO.Nth(8)}, nil},
		{"monthly_5", MONTHLY, nil, []Weekday{MO.Nth(5)}, []string{"2026-03-30T10:00:00Z", "2026-06-29T10:00:00Z"}},
		{"monthly_negative5", MONTHLY, nil, []Weekday{MO.Nth(-5)}, []string{"2026-03-02T10:00:00Z", "2026-06-01T10:00:00Z"}},
		{"monthly_mixed", MONTHLY, nil, []Weekday{MO.Nth(-6), MO.Nth(5), MO.Nth(53)}, []string{"2026-03-30T10:00:00Z", "2026-06-29T10:00:00Z"}},
		{"yearly_53", YEARLY, nil, []Weekday{MO.Nth(53)}, []string{"2029-12-31T10:00:00Z", "2035-12-31T10:00:00Z"}},
		{"yearly_negative53", YEARLY, nil, []Weekday{MO.Nth(-53)}, []string{"2029-01-01T10:00:00Z", "2035-01-01T10:00:00Z"}},
		{"weekly_ignored", WEEKLY, nil, []Weekday{MO.Nth(53)}, []string{"2026-01-05T10:00:00Z", "2026-01-12T10:00:00Z"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			safetyOccurrences(t, ROption{Freq: tc.freq, Dtstart: start, Count: 2, Bymonth: tc.month, Byweekday: tc.days}, tc.want...)
		})
	}
}

func TestSafetyEaster(t *testing.T) {
	maxInt := int(^uint(0) >> 1)
	for _, offset := range []int{-100, 300, -maxInt - 1, maxInt} {
		t.Run(fmt.Sprint(offset), func(t *testing.T) {
			option := ROption{Freq: YEARLY, Dtstart: safetyDate("2026-01-05T10:00:00Z"), Count: 2, Until: safetyDate("2027-12-31T23:59:59Z"), Byeaster: []int{offset}}
			safetyOccurrences(t, option)
			option.Byeaster = []int{0, offset}
			safetyOccurrences(t, option, "2026-04-05T10:00:00Z", "2027-03-28T10:00:00Z")
		})
	}
	for _, tc := range []struct {
		name, start string
		offset      int
		want        []string
	}{
		{"early_boundary", "1818-01-01T10:00:00Z", -81, nil},
		{"early_valid", "1818-01-01T10:00:00Z", -80, []string{"1818-01-01T10:00:00Z"}},
		{"late_boundary", "1943-01-01T10:00:00Z", 258, nil},
		{"late_spill_drop", "1943-01-01T10:00:00Z", 257, nil},
		{"nearby_257_valid", "1944-01-01T10:00:00Z", 257, []string{"1944-12-22T10:00:00Z"}},
		{"ordinary", "2026-01-05T10:00:00Z", -1, []string{"2026-04-04T10:00:00Z"}},
		{"negative80", "2026-01-05T10:00:00Z", -80, []string{"2026-01-15T10:00:00Z"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			start := safetyDate(tc.start)
			// Empty matches are bounded to the fixture year, but a single Next
			// still scans to MAXYEAR under the existing UNTIL scan limitation.
			safetyOccurrences(t, ROption{Freq: YEARLY, Dtstart: start, Until: time.Date(start.Year(), 12, 31, 23, 59, 59, 0, time.UTC), Count: 1, Byeaster: []int{tc.offset}}, tc.want...)
		})
	}
}

func TestSafetyInterval(t *testing.T) {
	maxInt := int(^uint(0) >> 1)
	for _, freq := range []Frequency{YEARLY, MONTHLY, WEEKLY, DAILY, HOURLY, MINUTELY, SECONDLY} {
		for _, count := range []int{1, 3} {
			t.Run(fmt.Sprintf("%s/count%d", freq, count), func(t *testing.T) {
				// Close enough to MAXYEAR that even 32-bit MaxInt seconds
				// must exhaust. COUNT>1 forces advancement beyond DTSTART.
				safetyOccurrences(t, ROption{Freq: freq, Dtstart: safetyDate("9999-12-01T10:00:00Z"), Interval: maxInt, Count: count}, "9999-12-01T10:00:00Z")
			})
		}
	}
	if strconv.IntSize == 64 {
		for _, freq := range []Frequency{MONTHLY, DAILY, HOURLY} {
			t.Run("original_"+freq.String(), func(t *testing.T) {
				safetyOccurrences(t, ROption{Freq: freq, Dtstart: safetyDate("2026-01-05T10:00:00Z"), Interval: maxInt, Count: 3}, "2026-01-05T10:00:00Z")
			})
		}
	}
}

func TestSafetyIntervalControls(t *testing.T) {
	start := safetyDate("2026-01-05T10:00:00Z")
	for _, tc := range []struct {
		freq     Frequency
		interval int
		second   string
	}{
		{YEARLY, 2, "2028-01-05T10:00:00Z"},
		{MONTHLY, 13, "2027-02-05T10:00:00Z"},
		{WEEKLY, 2, "2026-01-19T10:00:00Z"},
		{DAILY, 400, "2027-02-09T10:00:00Z"},
		{HOURLY, 25, "2026-01-06T11:00:00Z"},
		{MINUTELY, 1441, "2026-01-06T10:01:00Z"},
		{SECONDLY, 86401, "2026-01-06T10:00:01Z"},
		{YEARLY, 7973, "9999-01-05T10:00:00Z"},
		{MONTHLY, 95687, "9999-12-05T10:00:00Z"},
		// Independently calculated with Gregorian dates and seconds/minutes,
		// not the iterator's normalization. Valid on 32-bit as well as 64-bit.
		{SECONDLY, 2147483647, "2094-01-23T13:14:07Z"},
		{MINUTELY, 2147483647, "6109-01-28T12:07:00Z"},
	} {
		t.Run(fmt.Sprintf("%s/%d", tc.freq, tc.interval), func(t *testing.T) {
			safetyOccurrences(t, ROption{Freq: tc.freq, Dtstart: start, Interval: tc.interval, Count: 2, Until: safetyDate("9999-12-31T23:59:59Z")}, "2026-01-05T10:00:00Z", tc.second)
		})
	}
	for _, freq := range []Frequency{YEARLY, MONTHLY, WEEKLY, DAILY, HOURLY, MINUTELY, SECONDLY} {
		t.Run("near_end_"+freq.String(), func(t *testing.T) {
			safetyOccurrences(t, ROption{Freq: freq, Dtstart: safetyDate("9999-12-31T23:59:59Z"), Count: 2}, "9999-12-31T23:59:59Z")
		})
	}
	for _, interval := range []int{0, 1} {
		t.Run(fmt.Sprintf("default_%d", interval), func(t *testing.T) {
			safetyOccurrences(t, ROption{Freq: DAILY, Dtstart: start, Count: 2, Interval: interval}, "2026-01-05T10:00:00Z", "2026-01-06T10:00:00Z")
		})
	}
	bad := ROption{Freq: DAILY, Dtstart: start, Interval: -1}
	if _, err := NewRRule(bad); err == nil {
		t.Fatal("negative option interval accepted")
	}
	if _, err := StrToRRule(bad.String()); err == nil {
		t.Fatal("negative text interval accepted")
	}
	if strconv.IntSize == 32 {
		if _, err := StrToRRule("DTSTART:20260105T100000Z\nRRULE:FREQ=SECONDLY;INTERVAL=9223372036854775807"); err == nil {
			t.Fatal("unrepresentable interval accepted")
		}
	}
}

func TestSafetyBuffered(t *testing.T) {
	for _, freq := range []Frequency{YEARLY, MONTHLY, WEEKLY, DAILY} {
		t.Run(freq.String(), func(t *testing.T) {
			safetyOccurrences(t, ROption{Freq: freq, Dtstart: safetyDate("9999-12-27T10:00:00Z"), Interval: int(^uint(0) >> 1), Count: 4, Byhour: []int{10, 11}, Wkst: SU}, "9999-12-27T10:00:00Z", "9999-12-27T11:00:00Z")
		})
	}
}

func TestSafetySubdailyBuffered(t *testing.T) {
	safetyOccurrences(t, ROption{Freq: HOURLY, Dtstart: safetyDate("9999-12-31T10:00:00Z"), Interval: int(^uint(0) >> 1), Count: 4, Byminute: []int{0, 30}}, "9999-12-31T10:00:00Z", "9999-12-31T10:30:00Z")
	safetyOccurrences(t, ROption{Freq: MINUTELY, Dtstart: safetyDate("9999-12-31T10:00:00Z"), Interval: int(^uint(0) >> 1), Count: 4, Bysecond: []int{0, 30}}, "9999-12-31T10:00:00Z", "9999-12-31T10:00:30Z")
}

func TestSafetyWeeklyAlignment(t *testing.T) {
	safetyOccurrences(t, ROption{Freq: WEEKLY, Dtstart: safetyDate("2026-01-08T10:00:00Z"), Interval: 2, Wkst: SU, Count: 3, Byweekday: []Weekday{MO, TH}}, "2026-01-08T10:00:00Z", "2026-01-19T10:00:00Z", "2026-01-22T10:00:00Z")
}

func TestSafetyInnerCarries(t *testing.T) {
	for _, tc := range []struct {
		freq                 Frequency
		interval             int
		hour, minute, second []int
	}{
		{HOURLY, 25, []int{9}, nil, nil},
		{MINUTELY, 1441, nil, []int{9}, nil},
		{SECONDLY, 86401, nil, nil, []int{9}},
	} {
		t.Run(tc.freq.String(), func(t *testing.T) {
			safetyOccurrences(t, ROption{Freq: tc.freq, Dtstart: safetyDate("9999-12-30T00:00:00Z"), Count: 2, Interval: tc.interval, Byhour: tc.hour, Byminute: tc.minute, Bysecond: tc.second})
		})
	}
	// The filtered-day fast path can temporarily leave minutes or seconds
	// above their usual range before the next interval is added.
	for _, tc := range []struct {
		freq          Frequency
		first, second string
	}{
		{HOURLY, "9999-12-31T00:20:30Z", "9999-12-31T01:20:30Z"},
		{MINUTELY, "9999-12-31T00:00:30Z", "9999-12-31T00:01:30Z"},
		{SECONDLY, "9999-12-31T00:00:00Z", "9999-12-31T00:00:01Z"},
	} {
		t.Run("filtered_"+tc.freq.String(), func(t *testing.T) {
			safetyOccurrences(t, ROption{Freq: tc.freq, Dtstart: safetyDate("9999-12-30T10:20:30Z"), Count: 2, Interval: 1, Bymonthday: []int{31}}, tc.first, tc.second)
		})
	}
	for _, tc := range []struct {
		freq                 Frequency
		interval             int
		hour, minute, second []int
	}{
		{HOURLY, 24, []int{1}, nil, nil},
		{MINUTELY, 1440, nil, []int{1}, nil},
		{SECONDLY, 86400, nil, nil, []int{1}},
	} {
		t.Run("horizon_"+tc.freq.String(), func(t *testing.T) {
			safetyOccurrences(t, ROption{Freq: tc.freq, Dtstart: safetyDate("9999-12-30T00:00:00Z"), Count: 2, Interval: tc.interval, Byhour: tc.hour, Byminute: tc.minute, Bysecond: tc.second})
		})
	}
}

func TestSafetyWallClock(t *testing.T) {
	loc, err := time.LoadLocation("America/New_York")
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		freq     Frequency
		interval int
		second   string
	}{
		{DAILY, 1, "2026-03-08T10:00:00-04:00"},
		{HOURLY, 24, "2026-03-08T10:00:00-04:00"},
		{MINUTELY, 1440, "2026-03-08T10:00:00-04:00"},
		{SECONDLY, 86400, "2026-03-08T10:00:00-04:00"},
	} {
		t.Run(tc.freq.String(), func(t *testing.T) {
			safetyOccurrences(t, ROption{Freq: tc.freq, Dtstart: time.Date(2026, 3, 7, 10, 0, 0, 0, loc), Interval: tc.interval, Count: 2}, "2026-03-07T10:00:00-05:00", tc.second)
		})
	}
}

func TestSafetyLarge32BitIntervals(t *testing.T) {
	start := safetyDate("2026-01-05T10:00:00Z")
	safetyOccurrences(t, ROption{Freq: SECONDLY, Dtstart: start, Interval: 2147483647, Count: 3}, "2026-01-05T10:00:00Z", "2094-01-23T13:14:07Z", "2162-02-11T16:28:14Z")
	safetyOccurrences(t, ROption{Freq: MINUTELY, Dtstart: start, Interval: 2147483647, Count: 3, Until: safetyDate("9999-12-31T23:59:59Z")}, "2026-01-05T10:00:00Z", "6109-01-28T12:07:00Z")
}
