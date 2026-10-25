> **This is a maintained fork of [teambition/rrule-go](https://github.com/teambition/rrule-go).**
> The original project is no longer maintained. This fork keeps the same module path
> (`github.com/teambition/rrule-go`), the same API and the same Go 1.16 minimum. It adds fixes for
> inputs the parser accepts but the iterator could not handle.

## What this fork fixes (v1.8.3)

- **Panics on accepted rules.** BYDAY ordinals outside the month, BYEASTER offsets outside the year,
  and very large INTERVAL values could make the iterator panic with "index out of range". They now
  either yield no occurrence for the impossible parts or stop safely.
- **A hang.** HOURLY, MINUTELY and SECONDLY rules whose BYHOUR, BYMINUTE or BYSECOND values can't be
  reached from DTSTART at the given INTERVAL looped forever. They now finish with no occurrence.
- **Long scans with UNTIL (UTC only).** When DTSTART is in UTC, a rule that never matches now stops
  at UNTIL instead of scanning to the year 9999. Other locations behave as before.

Rules that worked before produce the same occurrences. The fixes come with regression tests and a
fuzz test.

## Using it

Keep importing `github.com/teambition/rrule-go` and point the module at this fork:

```bash
go mod edit -replace github.com/teambition/rrule-go=github.com/kentonium3/rrule-go@v1.8.3
go mod tidy
```

Report problems in this repository's issues. Licensed under MIT, as the original (© Teambition).

---

# rrule-go

Go library for working with recurrence rules for calendar dates.

[![CI](https://github.com/kentonium3/rrule-go/actions/workflows/ci.yml/badge.svg)](https://github.com/kentonium3/rrule-go/actions/workflows/ci.yml)
[![CodeQL](https://github.com/kentonium3/rrule-go/actions/workflows/codeql.yml/badge.svg)](https://github.com/kentonium3/rrule-go/actions/workflows/codeql.yml)
[![License](https://img.shields.io/badge/license-mit-blue.svg?style=flat-square)](https://github.com/kentonium3/rrule-go/blob/release/LICENSE)
[![Go Reference](https://pkg.go.dev/badge/github.com/teambition/rrule-go.svg)](https://pkg.go.dev/github.com/teambition/rrule-go)

The rrule module offers a complete implementation of the recurrence rules documented in the [iCalendar
RFC](http://www.ietf.org/rfc/rfc2445.txt). It is a partial port of the rrule module from the excellent [python-dateutil](http://labix.org/python-dateutil/) library.

## Demo

### rrule.RRule

```go
package main

import (
  "fmt"
  "time"

  "github.com/teambition/rrule-go"
)

func printTimeSlice(ts []time.Time) {
	for _, t := range ts {
		fmt.Println(t)
	}
}

func main() {
	// Daily, for 10 occurrences.
	r, _ := rrule.NewRRule(rrule.ROption{
		Freq:    rrule.DAILY,
		Count:   10,
		Dtstart: time.Date(1997, 9, 2, 9, 0, 0, 0, time.UTC),
	})

	fmt.Println(r.String())
	// DTSTART:19970902T090000Z
	// RRULE:FREQ=DAILY;COUNT=10

	printTimeSlice(r.All())
	// 1997-09-02 09:00:00 +0000 UTC
	// 1997-09-03 09:00:00 +0000 UTC
	// ...
	// 1997-09-07 09:00:00 +0000 UTC

	printTimeSlice(r.Between(
		time.Date(1997, 9, 6, 0, 0, 0, 0, time.UTC),
		time.Date(1997, 9, 8, 0, 0, 0, 0, time.UTC), true))
	// [1997-09-06 09:00:00 +0000 UTC
	//  1997-09-07 09:00:00 +0000 UTC]

	// Every four years, the first Tuesday after a Monday in November, 3 occurrences (U.S. Presidential Election day).
	r, _ = rrule.NewRRule(rrule.ROption{
		Freq:       rrule.YEARLY,
		Interval:   4,
		Count:      3,
		Bymonth:    []int{11},
		Byweekday:  []rrule.Weekday{rrule.TU},
		Bymonthday: []int{2, 3, 4, 5, 6, 7, 8},
		Dtstart:    time.Date(1996, 11, 5, 9, 0, 0, 0, time.UTC),
	})

	fmt.Println(r.String())
	// DTSTART:19961105T090000Z
	// RRULE:FREQ=YEARLY;INTERVAL=4;COUNT=3;BYMONTH=11;BYMONTHDAY=2,3,4,5,6,7,8;BYDAY=TU

	printTimeSlice(r.All())
	// 1996-11-05 09:00:00 +0000 UTC
	// 2000-11-07 09:00:00 +0000 UTC
	// 2004-11-02 09:00:00 +0000 UTC
}

```

### rrule.Set

```go
func ExampleSet() {
	// Daily, for 7 days, jumping Saturday and Sunday occurrences.
	set := rrule.Set{}
	r, _ := rrule.NewRRule(rrule.ROption{
		Freq:    rrule.DAILY,
		Count:   7,
		Dtstart: time.Date(1997, 9, 2, 9, 0, 0, 0, time.UTC)})
	set.RRule(r)

	fmt.Println(set.String())
	// DTSTART:19970902T090000Z
	// RRULE:FREQ=DAILY;COUNT=7

	printTimeSlice(set.All())
	// 1997-09-02 09:00:00 +0000 UTC
	// 1997-09-03 09:00:00 +0000 UTC
	// 1997-09-04 09:00:00 +0000 UTC
	// 1997-09-05 09:00:00 +0000 UTC
	// 1997-09-06 09:00:00 +0000 UTC
	// 1997-09-07 09:00:00 +0000 UTC
	// 1997-09-08 09:00:00 +0000 UTC

	// Weekly, for 4 weeks, plus one time on day 7, and not on day 16.
	set = rrule.Set{}
	r, _ = rrule.NewRRule(rrule.ROption{
		Freq:    rrule.WEEKLY,
		Count:   4,
		Dtstart: time.Date(1997, 9, 2, 9, 0, 0, 0, time.UTC)})
	set.RRule(r)
	set.RDate(time.Date(1997, 9, 7, 9, 0, 0, 0, time.UTC))
	set.ExDate(time.Date(1997, 9, 16, 9, 0, 0, 0, time.UTC))

	fmt.Println(set.String())
	// DTSTART:19970902T090000Z
	// RRULE:FREQ=WEEKLY;COUNT=4
	// RDATE:19970907T090000Z
	// EXDATE:19970916T090000Z

	printTimeSlice(set.All())
	// 1997-09-02 09:00:00 +0000 UTC
	// 1997-09-07 09:00:00 +0000 UTC
	// 1997-09-09 09:00:00 +0000 UTC
	// 1997-09-23 09:00:00 +0000 UTC
}
```

### rrule.StrToRRule

```go
func ExampleStrToRRule() {
	// Compatible with old DTSTART
	r, _ := rrule.StrToRRule("FREQ=DAILY;DTSTART=20060101T150405Z;COUNT=5")
	fmt.Println(r.OrigOptions.RRuleString())
	// FREQ=DAILY;COUNT=5

	fmt.Println(r.OrigOptions.String())
	// DTSTART:20060101T150405Z
	// RRULE:FREQ=DAILY;COUNT=5

	fmt.Println(r.String())
	// DTSTART:20060101T150405Z
	// RRULE:FREQ=DAILY;COUNT=5

	printTimeSlice(r.All())
	// 2006-01-01 15:04:05 +0000 UTC
	// 2006-01-02 15:04:05 +0000 UTC
	// 2006-01-03 15:04:05 +0000 UTC
	// 2006-01-04 15:04:05 +0000 UTC
	// 2006-01-05 15:04:05 +0000 UTC
}
```

### rrule.StrToRRuleSet

```go
func ExampleStrToRRuleSet() {
	s, _ := rrule.StrToRRuleSet("DTSTART:20060101T150405Z\nRRULE:FREQ=DAILY;COUNT=5\nEXDATE:20060102T150405Z")
	fmt.Println(s.String())
	// DTSTART:20060101T150405Z
	// RRULE:FREQ=DAILY;COUNT=5
	// EXDATE:20060102T150405Z

	printTimeSlice(s.All())
	// 2006-01-01 15:04:05 +0000 UTC
	// 2006-01-03 15:04:05 +0000 UTC
	// 2006-01-04 15:04:05 +0000 UTC
	// 2006-01-05 15:04:05 +0000 UTC
}
```

For more examples see [python-dateutil](http://labix.org/python-dateutil/) documentation.

## License

Gear is licensed under the [MIT](https://github.com/teambition/gear/blob/master/LICENSE) license.
Copyright &copy; 2017-2023 [Teambition](https://www.teambition.com).
