package rrule

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"reflect"
	"strconv"
	"strings"
	"testing"
	"time"
)

// The expensive negative runs in a child so a failed regression cannot leave a
// CPU-bound iterator behind. CommandContext kills it and Output always waits.
func TestUntilScan(t *testing.T) {
	if os.Getenv("RRULE_UNTIL_CHILD") != "" {
		untilScanChild(t)
		return
	}
	for _, constructor := range []string{"new", "parse"} {
		for _, operation := range []string{"iterator", "all", "after"} {
			for _, positions := range []string{"1", "2", "heavy"} {
				t.Run(constructor+"/"+operation+"/"+positions, func(t *testing.T) {
					ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
					defer cancel()
					cmd := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestUntilScan$", "-test.v")
					cmd.Env = append(os.Environ(), "RRULE_UNTIL_CHILD="+constructor+","+operation+","+positions)
					output, err := cmd.CombinedOutput()
					if err != nil {
						t.Fatalf("child: %v (watchdog: %v)\n%s", err, ctx.Err(), output)
					}
					t.Log(strings.TrimSpace(string(output)))
				})
			}
		}
	}
}

func untilScanChild(t *testing.T) {
	parts := strings.Split(os.Getenv("RRULE_UNTIL_CHILD"), ",")
	positions := []int{1}
	if parts[2] == "2" {
		positions = []int{2}
	}
	if parts[2] == "heavy" {
		positions = nil
		for i := 2; i <= 366; i++ {
			positions = append(positions, i)
		}
	}
	start := time.Date(2026, 1, 5, 10, 0, 0, 0, time.UTC)
	opt := ROption{Freq: DAILY, Dtstart: start, Until: time.Date(2026, 2, 1, 0, 0, 0, 0, time.UTC), Bysetpos: positions}
	var r *RRule
	var err error
	if parts[0] == "new" {
		r, err = NewRRule(opt)
	} else {
		values := []string{}
		for _, n := range positions {
			values = append(values, strconv.Itoa(n))
		}
		r, err = StrToRRule("DTSTART:20260105T100000Z\nRRULE:FREQ=DAILY;UNTIL=20260201T000000Z;BYSETPOS=" + strings.Join(values, ","))
	}
	if err != nil {
		t.Fatal(err)
	}
	begin := time.Now()
	count := 0
	switch parts[1] {
	case "iterator":
		next := r.Iterator()
		for {
			_, ok := next()
			if !ok {
				break
			}
			count++
		}
		if _, ok := next(); ok {
			t.Fatal("iterator revived after exhaustion")
		}
	case "all":
		count = len(r.All())
	case "after":
		if !r.After(start, true).IsZero() {
			count = 1
		}
	}
	elapsed := time.Since(begin)
	if parts[2] == "1" {
		want := 27
		if parts[1] == "after" {
			want = 1
		}
		if count != want {
			t.Fatalf("positive control count %d, want %d", count, want)
		}
	} else if count != 0 {
		t.Fatalf("nonmatching result count %d", count)
	}
	t.Logf("%s operation=%s", os.Getenv("RRULE_UNTIL_CHILD"), elapsed)
	if elapsed >= 100*time.Millisecond {
		t.Fatalf("operation %s exceeds 100ms", elapsed)
	}
}

func untilStrings(r *RRule) []string {
	out := []string{}
	for _, v := range r.All() {
		out = append(out, v.Format(time.RFC3339))
	}
	return out
}

func untilGoldenCases(t *testing.T) map[string]ROption {
	utc := func(y int, m time.Month, d, h int) time.Time { return time.Date(y, m, d, h, 0, 0, 0, time.UTC) }
	cases := map[string]ROption{}
	for f := YEARLY; f <= SECONDLY; f++ {
		start := utc(2026, 1, 5, 10)
		end := start
		switch f {
		case YEARLY:
			end = utc(2028, 1, 5, 10)
		case MONTHLY:
			end = utc(2026, 3, 5, 10)
		case WEEKLY:
			end = utc(2026, 1, 19, 10)
		case DAILY:
			end = utc(2026, 1, 7, 10)
		case HOURLY:
			end = start.Add(2 * time.Hour)
		case MINUTELY:
			end = start.Add(2 * time.Minute)
		case SECONDLY:
			end = start.Add(2 * time.Second)
		}
		for _, delta := range []time.Duration{-time.Second, 0, time.Second} {
			cases[fmt.Sprintf("frequency-%d-boundary-%d", f, delta)] = ROption{Freq: f, Dtstart: start, Until: end.Add(delta)}
		}
	}
	cases["year-earlier-month"] = ROption{Freq: YEARLY, Dtstart: utc(2026, 11, 30, 10), Until: utc(2027, 2, 1, 10), Bymonth: []int{1}, Bymonthday: []int{1}}
	cases["month-earlier-day"] = ROption{Freq: MONTHLY, Dtstart: utc(2026, 1, 31, 10), Until: utc(2026, 2, 2, 10), Bymonthday: []int{1}}
	cases["partial-cross-year-week"] = ROption{Freq: WEEKLY, Dtstart: utc(2025, 12, 30, 10), Until: utc(2026, 1, 12, 10), Wkst: SU, Byweekday: []Weekday{MO, WE, SU}}
	cases["count-buffer"] = ROption{Freq: MONTHLY, Dtstart: utc(2026, 1, 1, 10), Until: utc(2026, 3, 1, 10), Count: 4, Bymonthday: []int{1, 2, 3}, Byhour: []int{9, 10}, Bysetpos: []int{1, 2, -1}}
	cases["empty-dayset"] = ROption{Freq: YEARLY, Dtstart: utc(9998, 1, 1, 10), Until: utc(9998, 2, 1, 10), Bymonth: []int{2}, Bymonthday: []int{30}}
	cases["different-until-offset"] = ROption{Freq: DAILY, Dtstart: utc(2026, 1, 5, 10), Until: time.Date(2026, 1, 7, 13, 0, 0, 0, time.FixedZone("east", 3*3600))}
	for name, spec := range map[string]struct {
		zone string
		y    int
		m    time.Month
		d    int
	}{"sao-paulo": {"America/Sao_Paulo", 2018, 11, 3}, "apia": {"Pacific/Apia", 2011, 12, 29}, "gap": {"America/New_York", 2026, 3, 7}, "fold": {"America/New_York", 2026, 10, 31}} {
		loc, err := time.LoadLocation(spec.zone)
		if err != nil {
			t.Fatal(err)
		}
		start := time.Date(spec.y, spec.m, spec.d, 0, 0, 0, 0, loc)
		end := start.Add(23*time.Hour + 59*time.Minute + 59*time.Second)
		if name == "gap" || name == "fold" {
			start = start.Add(90 * time.Minute)
			if name == "gap" {
				start = start.Add(time.Hour)
			}
			end = start.Add(72 * time.Hour)
		}
		cases[name] = ROption{Freq: DAILY, Dtstart: start, Until: end}
	}
	for _, offset := range []int{-72 * 3600, 72 * 3600, 3600} {
		loc := time.FixedZone("UTC", offset)
		start := time.Date(2026, 1, 5, 10, 0, 0, 0, loc)
		cases[fmt.Sprintf("named-UTC-%d", offset)] = ROption{Freq: DAILY, Dtstart: start, Until: start.Add(48 * time.Hour)}
	}
	return cases
}

func TestUntilGolden(t *testing.T) {
	got := map[string][]string{}
	for name, opt := range untilGoldenCases(t) {
		r, err := NewRRule(opt)
		if err != nil {
			t.Fatal(err)
		}
		got[name] = untilStrings(r)
	}
	if os.Getenv("RRULE_CAPTURE_GOLDEN") != "" {
		data, _ := json.MarshalIndent(got, "", "  ")
		if err := os.WriteFile(os.Getenv("RRULE_CAPTURE_GOLDEN"), data, 0600); err != nil {
			t.Fatal(err)
		}
		return
	}
	var want map[string][]string
	if err := json.Unmarshal([]byte(untilBaselineGolden), &want); err != nil {
		t.Fatal(err)
	}
	for name, values := range got {
		if !reflect.DeepEqual(values, want[name]) {
			t.Errorf("%s got %v want %v", name, values, want[name])
		}
	}
	if len(got) != len(want) {
		t.Fatal("golden fixture count changed")
	}
}

const untilBaselineGolden = `{
  "apia": [
    "2011-12-29T00:00:00-10:00",
    "2011-12-29T00:00:00-10:00"
  ],
  "count-buffer": [
    "2026-01-01T10:00:00Z",
    "2026-01-03T10:00:00Z",
    "2026-02-01T09:00:00Z",
    "2026-02-01T10:00:00Z"
  ],
  "different-until-offset": [
    "2026-01-05T10:00:00Z",
    "2026-01-06T10:00:00Z",
    "2026-01-07T10:00:00Z"
  ],
  "empty-dayset": [],
  "fold": [
    "2026-10-31T01:30:00-04:00",
    "2026-11-01T01:30:00-04:00",
    "2026-11-02T01:30:00-05:00"
  ],
  "frequency-0-boundary--1000000000": [
    "2026-01-05T10:00:00Z",
    "2027-01-05T10:00:00Z"
  ],
  "frequency-0-boundary-0": [
    "2026-01-05T10:00:00Z",
    "2027-01-05T10:00:00Z",
    "2028-01-05T10:00:00Z"
  ],
  "frequency-0-boundary-1000000000": [
    "2026-01-05T10:00:00Z",
    "2027-01-05T10:00:00Z",
    "2028-01-05T10:00:00Z"
  ],
  "frequency-1-boundary--1000000000": [
    "2026-01-05T10:00:00Z",
    "2026-02-05T10:00:00Z"
  ],
  "frequency-1-boundary-0": [
    "2026-01-05T10:00:00Z",
    "2026-02-05T10:00:00Z",
    "2026-03-05T10:00:00Z"
  ],
  "frequency-1-boundary-1000000000": [
    "2026-01-05T10:00:00Z",
    "2026-02-05T10:00:00Z",
    "2026-03-05T10:00:00Z"
  ],
  "frequency-2-boundary--1000000000": [
    "2026-01-05T10:00:00Z",
    "2026-01-12T10:00:00Z"
  ],
  "frequency-2-boundary-0": [
    "2026-01-05T10:00:00Z",
    "2026-01-12T10:00:00Z",
    "2026-01-19T10:00:00Z"
  ],
  "frequency-2-boundary-1000000000": [
    "2026-01-05T10:00:00Z",
    "2026-01-12T10:00:00Z",
    "2026-01-19T10:00:00Z"
  ],
  "frequency-3-boundary--1000000000": [
    "2026-01-05T10:00:00Z",
    "2026-01-06T10:00:00Z"
  ],
  "frequency-3-boundary-0": [
    "2026-01-05T10:00:00Z",
    "2026-01-06T10:00:00Z",
    "2026-01-07T10:00:00Z"
  ],
  "frequency-3-boundary-1000000000": [
    "2026-01-05T10:00:00Z",
    "2026-01-06T10:00:00Z",
    "2026-01-07T10:00:00Z"
  ],
  "frequency-4-boundary--1000000000": [
    "2026-01-05T10:00:00Z",
    "2026-01-05T11:00:00Z"
  ],
  "frequency-4-boundary-0": [
    "2026-01-05T10:00:00Z",
    "2026-01-05T11:00:00Z",
    "2026-01-05T12:00:00Z"
  ],
  "frequency-4-boundary-1000000000": [
    "2026-01-05T10:00:00Z",
    "2026-01-05T11:00:00Z",
    "2026-01-05T12:00:00Z"
  ],
  "frequency-5-boundary--1000000000": [
    "2026-01-05T10:00:00Z",
    "2026-01-05T10:01:00Z"
  ],
  "frequency-5-boundary-0": [
    "2026-01-05T10:00:00Z",
    "2026-01-05T10:01:00Z",
    "2026-01-05T10:02:00Z"
  ],
  "frequency-5-boundary-1000000000": [
    "2026-01-05T10:00:00Z",
    "2026-01-05T10:01:00Z",
    "2026-01-05T10:02:00Z"
  ],
  "frequency-6-boundary--1000000000": [
    "2026-01-05T10:00:00Z",
    "2026-01-05T10:00:01Z"
  ],
  "frequency-6-boundary-0": [
    "2026-01-05T10:00:00Z",
    "2026-01-05T10:00:01Z",
    "2026-01-05T10:00:02Z"
  ],
  "frequency-6-boundary-1000000000": [
    "2026-01-05T10:00:00Z",
    "2026-01-05T10:00:01Z",
    "2026-01-05T10:00:02Z",
    "2026-01-05T10:00:03Z"
  ],
  "gap": [
    "2026-03-07T02:30:00-05:00",
    "2026-03-08T01:30:00-05:00",
    "2026-03-09T02:30:00-04:00",
    "2026-03-10T02:30:00-04:00"
  ],
  "month-earlier-day": [
    "2026-02-01T10:00:00Z"
  ],
  "named-UTC--259200": [
    "2026-01-05T10:00:00-72:00",
    "2026-01-06T10:00:00-72:00",
    "2026-01-07T10:00:00-72:00"
  ],
  "named-UTC-259200": [
    "2026-01-05T10:00:00+72:00",
    "2026-01-06T10:00:00+72:00",
    "2026-01-07T10:00:00+72:00"
  ],
  "named-UTC-3600": [
    "2026-01-05T10:00:00+01:00",
    "2026-01-06T10:00:00+01:00",
    "2026-01-07T10:00:00+01:00"
  ],
  "partial-cross-year-week": [
    "2025-12-31T10:00:00Z",
    "2026-01-04T10:00:00Z",
    "2026-01-05T10:00:00Z",
    "2026-01-07T10:00:00Z",
    "2026-01-11T10:00:00Z",
    "2026-01-12T10:00:00Z"
  ],
  "sao-paulo": [
    "2018-11-03T00:00:00-03:00",
    "2018-11-03T00:00:00-03:00"
  ],
  "year-earlier-month": [
    "2027-01-01T10:00:00Z"
  ]
}`

func TestUntilSetters(t *testing.T) {
	start := time.Date(2026, 1, 5, 10, 0, 0, 0, time.UTC)
	r, err := NewRRule(ROption{Freq: DAILY, Dtstart: start, Until: start.Add(48 * time.Hour)})
	if err != nil {
		t.Fatal(err)
	}
	check := func(want int) {
		t.Helper()
		if got := len(r.All()); got != want {
			t.Fatalf("got %d occurrences, want %d", got, want)
		}
	}
	check(3)
	r.Until(start)
	check(1)
	r.Until(start.Add(72 * time.Hour))
	check(4)
	r.DTStart(start.Add(24 * time.Hour))
	check(3)
	local := time.Date(2026, 1, 6, 10, 0, 0, 0, time.FixedZone("UTC", 3600))
	r.DTStart(local)
	r.Until(local.Add(24 * time.Hour))
	check(2)
	r.DTStart(start)
	r.Until(start.Add(24 * time.Hour))
	check(2)
	r.Until(time.Time{})
	if !r.GetUntil().Equal(start.Add(time.Duration(1<<63 - 1))) {
		t.Fatal("default horizon changed")
	}
	// A default-horizon nonmatch proves exhaustion without a candidate UNTIL check.
	empty, err := NewRRule(ROption{Freq: DAILY, Dtstart: start, Bysetpos: []int{2}})
	if err != nil {
		t.Fatal(err)
	}
	begin := time.Now()
	if got := empty.All(); len(got) != 0 {
		t.Fatal(got)
	}
	t.Logf("default horizon empty scan %s", time.Since(begin))
}

// Inspect where an empty DAILY iterator stops, so horizon coverage does not
// depend on machine speed or merely observing the same empty result at MAXYEAR.
func TestUntilEmptyStopsAtHorizon(t *testing.T) {
	start := time.Date(2026, 1, 5, 10, 0, 0, 0, time.UTC)
	r, err := NewRRule(ROption{Freq: DAILY, Dtstart: start, Bysetpos: []int{2}})
	if err != nil {
		t.Fatal(err)
	}
	check := func() {
		t.Helper()
		it := rIterator{ii: iterInfo{rrule: r}, timeset: r.timeset, count: r.count}
		it.year, it.month, it.day = r.dtstart.Date()
		it.hour, it.minute, it.second = r.dtstart.Clock()
		it.weekday = toPyWeekday(r.dtstart.Weekday())
		it.ii.rebuild(it.year, it.month)
		it.generate()
		got := time.Date(it.year, it.month, it.day, 0, 0, 0, 0, time.UTC)
		want := time.Date(r.until.Year(), r.until.Month(), r.until.Day()+1, 0, 0, 0, 0, time.UTC)
		if !it.finished || !got.Equal(want) {
			t.Fatalf("empty iterator finished=%v at %v, want %v", it.finished, got, want)
		}
		it.generate()
		if it.remain.Len() != 0 {
			t.Fatal("exhausted iterator gained results")
		}
	}
	check() // Existing default horizon, rather than MAXYEAR.
	r.Until(start.Add(24 * time.Hour))
	check()
	r.Until(start.Add(72 * time.Hour))
	check()
	r.DTStart(start.Add(24 * time.Hour))
	check()
	r.Until(time.Time{})
	check()
}
