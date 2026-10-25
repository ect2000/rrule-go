package rrule

import (
	"context"
	"os"
	"os/exec"
	"reflect"
	"strings"
	"testing"
	"time"
)

// Every fixture runs in a process that CommandContext kills and Wait reaps.
// The watchdog includes startup/race teardown; the operation has its own limit.
func TestClockReachability(t *testing.T) {
	cases := []struct {
		name, rule, start string
		want              []string
	}{
		{"hourly", "FREQ=HOURLY;INTERVAL=2;BYHOUR=1", "20260105T100000Z", nil},
		{"minutely", "FREQ=MINUTELY;INTERVAL=2;BYMINUTE=1", "20260105T100000Z", nil},
		{"secondly", "FREQ=SECONDLY;INTERVAL=2;BYSECOND=1", "20260105T100000Z", nil},
		{"until", "FREQ=HOURLY;INTERVAL=2;BYHOUR=1;UNTIL=20260201T000000Z", "20260105T100000Z", nil},
		{"hour-control", "FREQ=HOURLY;INTERVAL=2;BYHOUR=2;COUNT=2", "20260105T100000Z", []string{"20260106T020000Z", "20260107T020000Z"}},
		{"minute-control", "FREQ=MINUTELY;INTERVAL=2;BYMINUTE=2;COUNT=2", "20260105T100000Z", []string{"20260105T100200Z", "20260105T110200Z"}},
		{"second-control", "FREQ=SECONDLY;INTERVAL=2;BYSECOND=2;COUNT=2", "20260105T100000Z", []string{"20260105T100002Z", "20260105T100102Z"}},
		{"joint-minute", "FREQ=MINUTELY;INTERVAL=90;BYHOUR=11;BYMINUTE=0", "20260105T100000Z", nil},
		{"joint-second", "FREQ=SECONDLY;INTERVAL=90;BYMINUTE=1;BYSECOND=0", "20260105T100000Z", nil},
		{"mixed", "FREQ=HOURLY;INTERVAL=2;BYHOUR=1,2;COUNT=2", "20260105T100000Z", []string{"20260106T020000Z", "20260107T020000Z"}},
		{"local", "FREQ=HOURLY;INTERVAL=2;COUNT=2", "20261231T230000Z", []string{"20261231T230000Z", "20270101T010000Z"}},
		{"carry", "FREQ=SECONDLY;INTERVAL=2;COUNT=3", "20261231T235958Z", []string{"20261231T235958Z", "20270101T000000Z", "20270101T000002Z"}},
		{"filtered-hour", "FREQ=HOURLY;INTERVAL=5;BYDAY=TU;COUNT=2", "20260105T100000Z", []string{"20260106T010000Z", "20260106T060000Z"}},
		{"filtered-minute", "FREQ=MINUTELY;INTERVAL=65;BYDAY=TU;COUNT=2", "20260105T100000Z", []string{"20260106T000500Z", "20260106T011000Z"}},
		{"filtered-second", "FREQ=SECONDLY;INTERVAL=3601;BYDAY=TU;COUNT=2", "20260105T100000Z", []string{"20260106T000014Z", "20260106T010015Z"}},
		{"last-hour", "FREQ=HOURLY;BYHOUR=10;COUNT=2", "20260105T100000Z", []string{"20260105T100000Z", "20260106T100000Z"}},
		{"last-minute", "FREQ=MINUTELY;BYHOUR=10;BYMINUTE=0;COUNT=2", "20260105T100000Z", []string{"20260105T100000Z", "20260106T100000Z"}},
		{"last-second", "FREQ=SECONDLY;BYHOUR=10;BYMINUTE=0;BYSECOND=0;COUNT=2", "20260105T100000Z", []string{"20260105T100000Z", "20260106T100000Z"}},
		{"buffered", "FREQ=HOURLY;INTERVAL=24;BYHOUR=10;BYMINUTE=0,30;UNTIL=20260105T103000Z", "20260105T100000Z", []string{"20260105T100000Z", "20260105T103000Z"}},
		{"horizon", "FREQ=SECONDLY;INTERVAL=2;BYSECOND=1", "99991231T235958Z", nil},
	}
	for _, c := range cases {
		for _, constructor := range []string{"string", "option"} {
			for _, operation := range []string{"All", "After", "Iterator", "Setters"} {
				if operation == "Setters" && c.name != "hourly" && c.name != "minutely" && c.name != "secondly" {
					continue
				}
				name := c.name + "/" + constructor + "/" + operation
				t.Run(name, func(t *testing.T) {
					if os.Getenv("RRULE_CLOCK_CASE") != name {
						if os.Getenv("RRULE_CLOCK_CASE") != "" {
							t.SkipNow()
						}
						ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
						defer cancel()
						binary, err := os.Executable()
						if err != nil {
							t.Fatal(err)
						}
						cmd := exec.CommandContext(ctx, binary, "-test.run=^TestClockReachability$$", "-test.v")
						cmd.Env = append(os.Environ(), "RRULE_CLOCK_CASE="+name)
						output, err := cmd.CombinedOutput()
						if err != nil {
							t.Fatalf("child failed (watchdog=%v): %v\n%s", ctx.Err(), err, output)
						}
						for _, line := range strings.Split(string(output), "\n") {
							if strings.Contains(line, "operation=") {
								t.Log(line)
							}
						}
						return
					}
					began := time.Now()
					r, err := StrToRRule("DTSTART:" + c.start + "\nRRULE:" + c.rule)
					if err != nil {
						t.Fatal(err)
					}
					if constructor == "option" {
						r, err = NewRRule(r.OrigOptions)
						if err != nil {
							t.Fatal(err)
						}
					}
					if c.name == "local" {
						r.DTStart(r.GetDTStart().In(time.FixedZone("test", -5*60*60)))
					}
					var got []string
					switch operation {
					case "Setters":
						original := r.GetDTStart()
						if c.name == "hourly" || c.name == "minutely" || c.name == "secondly" {
							delta := time.Hour
							if c.name == "minutely" {
								delta = time.Minute
							}
							if c.name == "secondly" {
								delta = time.Second
							}
							reachable := original.Add(delta)
							cycle := 24 * time.Hour
							if c.name == "hourly" {
								reachable = original.Add(15 * time.Hour)
							}
							if c.name == "minutely" {
								cycle = time.Hour
							}
							if c.name == "secondly" {
								cycle = time.Minute
							}
							r.DTStart(reachable)
							if first := r.After(reachable.Add(-time.Second), false); !first.Equal(reachable) {
								t.Fatalf("reachable setter: %v", first)
							}
							r.DTStart(original)
							if values := r.All(); len(values) != 0 {
								t.Fatal(values)
							}
							r.Until(original.AddDate(0, 1, 0))
							if values := r.All(); len(values) != 0 {
								t.Fatal(values)
							}
							r.DTStart(reachable)
							r.Until(reachable)
							if values := r.All(); len(values) != 1 || !values[0].Equal(reachable) {
								t.Fatal(values)
							}
							r.Until(reachable.Add(cycle))
							values := r.All()
							if len(values) != 2 || !values[0].Equal(reachable) || !values[1].Equal(reachable.Add(cycle)) {
								t.Fatal(values)
							}
						}
						elapsed := time.Since(began)
						t.Logf("operation=%s elapsed=%s", name, elapsed)
						if elapsed > time.Second {
							t.Fatal("operation exceeded one second")
						}
						return

					case "All":
						for _, d := range r.All() {
							got = append(got, d.UTC().Format("20060102T150405Z"))
						}
					case "After":
						cursor := r.GetDTStart().Add(-time.Second)
						for i := 0; i <= len(c.want); i++ {
							d := r.After(cursor, false)
							if d.IsZero() {
								break
							}
							got = append(got, d.UTC().Format("20060102T150405Z"))
							cursor = d
						}
					case "Iterator":
						next := r.Iterator()
						for i := 0; i <= len(c.want); i++ {
							d, ok := next()
							if !ok {
								break
							}
							got = append(got, d.UTC().Format("20060102T150405Z"))
						}
						for i := 0; i < 2; i++ {
							if d, ok := next(); ok || !d.IsZero() {
								t.Fatal("not exhausted")
							}
						}
					}
					elapsed := time.Since(began)
					t.Logf("operation=%s elapsed=%s", name, elapsed)
					if elapsed > time.Second {
						t.Fatal("operation exceeded one second")
					}
					if !reflect.DeepEqual(got, c.want) {
						t.Fatalf("got %v want %v", got, c.want)
					}
				})
			}
		}
	}
}
