package main

import (
	"os"
	"strings"
	"testing"
	"time"
)

func day(t *testing.T, s string) time.Time {
	t.Helper()
	d, err := time.Parse(time.DateOnly, s)
	if err != nil {
		t.Fatal(err)
	}
	return d
}

func TestEaster(t *testing.T) {
	want := map[int]string{
		2024: "2024-03-31", 2025: "2025-04-20", 2026: "2026-04-05",
		2027: "2027-03-28", 2028: "2028-04-16", 2029: "2029-04-01",
		2030: "2030-04-21",
	}

	for y, w := range want {
		if got := easter(y).Format(time.DateOnly); got != w {
			t.Errorf("easter(%d) = %s, want %s", y, got, w)
		}
	}
}

func TestValidCalendar(t *testing.T) {
	for _, c := range []string{"PL", "DE-BY", "DE-TH"} {
		if !validCalendar(c) {
			t.Errorf("validCalendar(%q) = false", c)
		}
	}
	for _, c := range []string{"", "DE", "DE-XX", "de-by", "PL-MZ"} {
		if validCalendar(c) {
			t.Errorf("validCalendar(%q) = true", c)
		}
	}
}

// One case per rule, readable on its own; TestHolidaysGolden checks
// every day against the reference lists.
func TestShiftDeadline(t *testing.T) {
	cases := []struct{ cal, in, want, why string }{
		{"PL", "2026-11-20", "2026-11-20", "Friday, a working day"},
		{"PL", "2026-06-20", "2026-06-22", "Saturday"},
		{"PL", "2026-09-20", "2026-09-21", "Sunday"},
		{"PL", "2026-12-20", "2026-12-21", "Sunday"},
		{"PL", "2026-06-04", "2026-06-05", "Boze Cialo"},
		{"PL", "2026-12-24", "2026-12-28", "Wigilia, Boze Narodzenie, weekend"},
		{"PL", "2024-12-24", "2024-12-24", "Wigilia before 2025 is a working day"},
		{"PL", "2025-12-24", "2025-12-29", "Wigilia from 2025, Boze Narodzenie, weekend"},
		{"PL", "2027-01-06", "2027-01-07", "Trzech Kroli"},
		{"PL", "2027-02-20", "2027-02-22", "Saturday"},
		{"PL", "2027-03-29", "2027-03-30", "Poniedzialek Wielkanocny"},
		{"PL", "2027-05-01", "2027-05-04", "Swieto Pracy on a Saturday, Sunday, Swieto Konstytucji 3 Maja"},
		{"PL", "2026-04-03", "2026-04-03", "Wielki Piatek is a working day in Poland"},

		{"DE-BE", "2026-04-03", "2026-04-07", "Karfreitag, weekend, Ostermontag"},
		{"DE-BE", "2026-05-14", "2026-05-15", "Christi Himmelfahrt"},
		{"DE-BE", "2026-05-25", "2026-05-26", "Pfingstmontag"},
		{"DE-BE", "2026-10-03", "2026-10-05", "Tag der Deutschen Einheit, Saturday"},
		{"DE-BE", "2026-12-24", "2026-12-24", "Heiligabend is a working day in Germany"},
		{"DE-BE", "2026-11-11", "2026-11-11", "Narodowe Swieto Niepodleglosci is Polish only"},
		{"DE-BE", "2027-03-08", "2027-03-09", "Frauentag in Berlin"},
		{"DE-HH", "2027-03-08", "2027-03-08", "no Frauentag in Hamburg"},
		{"DE-BY", "2026-01-06", "2026-01-07", "Heilige Drei Koenige in Bavaria"},
		{"DE-NI", "2026-01-06", "2026-01-06", "no Heilige Drei Koenige in Lower Saxony"},
		{"DE-NW", "2026-06-04", "2026-06-05", "Fronleichnam in North Rhine-Westphalia"},
		{"DE-SN", "2026-06-04", "2026-06-04", "Fronleichnam only in some Saxon towns"},
		{"DE-SL", "2029-08-15", "2029-08-16", "Mariae Himmelfahrt in Saarland"},
		{"DE-BY", "2029-08-15", "2029-08-15", "only Catholic-majority Bavarian towns: +08-15"},
		{"DE-TH", "2028-09-20", "2028-09-21", "Weltkindertag in Thuringia"},
		{"DE-HB", "2028-10-31", "2028-11-01", "Reformationstag in Bremen"},
		{"DE-BW", "2028-10-31", "2028-10-31", "Baden-Wuerttemberg: school day off only"},
		{"DE-RP", "2027-11-01", "2027-11-02", "Allerheiligen in Rhineland-Palatinate"},
		{"DE-SN", "2026-11-18", "2026-11-19", "Buss- und Bettag in Saxony"},
		{"DE-BY", "2026-11-18", "2026-11-18", "Bavaria: school day off only"},
	}

	for _, c := range cases {
		if got := shiftDeadline(c.cal, day(t, c.in)).Format(time.DateOnly); got != c.want {
			t.Errorf("shiftDeadline(%s, %s) = %s, want %s (%s)", c.cal, c.in, got, c.want, c.why)
		}
	}
}

func TestBrandenburgSundays(t *testing.T) {
	for _, s := range []string{"2026-04-05", "2026-05-24"} { // Ostersonntag, Pfingstsonntag
		if !isHoliday("DE-BB", day(t, s)) || isHoliday("DE-BE", day(t, s)) {
			t.Errorf("%s: want a holiday in Brandenburg only", s)
		}
	}
}

// Every day of every year in testdata/holidays.txt, for every calendar.
func TestHolidaysGolden(t *testing.T) {
	b, err := os.ReadFile("testdata/holidays.txt")
	if err != nil {
		t.Fatal(err)
	}

	seen := map[string]bool{}
	for _, line := range strings.Split(strings.TrimSpace(string(b)), "\n") {
		if strings.HasPrefix(line, "#") {
			continue
		}

		f := strings.Fields(line)
		cal, year := f[0], f[1]
		seen[cal] = true

		want := map[string]bool{}
		for _, d := range f[2:] {
			want[year+"-"+d] = true
		}

		for d := day(t, year+"-01-01"); d.Format("2006") == year; d = d.AddDate(0, 0, 1) {
			if k := d.Format(time.DateOnly); isHoliday(cal, d) != want[k] {
				t.Errorf("%s %s: isHoliday = %v, reference says %v", cal, k, !want[k], want[k])
			}
		}
	}

	if len(seen) != 1+len(deStates) {
		t.Errorf("golden file covers %d calendars, want %d", len(seen), 1+len(deStates))
	}
}

func TestStateHolidaysByYear(t *testing.T) {
	cases := []struct {
		cal, date string
		want      bool
	}{
		{"DE-MV", "2022-03-08", false}, {"DE-MV", "2023-03-08", true},
		{"DE-BE", "2018-03-08", false}, {"DE-BE", "2019-03-08", true},
		{"DE-TH", "2018-09-20", false}, {"DE-TH", "2019-09-20", true},
		{"DE-NI", "2016-10-31", false}, {"DE-NI", "2018-10-31", true}, {"DE-SN", "2016-10-31", true},
	}

	for _, c := range cases {
		if got := isHoliday(c.cal, day(t, c.date)); got != c.want {
			t.Errorf("%s %s: got %v", c.cal, c.date, got)
		}
	}
}

func TestUnknownCalendarPanics(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Error("want a panic for calendar pl")
		}
	}()

	isHoliday("pl", day(t, "2026-12-25"))
}

func TestCalendarExtras(t *testing.T) {
	for cal, want := range map[string]bool{"DE-BY +08-15": true, "PL +12-31 +01-02": true,
		"DE-BY +8-15": false, "DE-BY 08-15": false, "+08-15": false, "DE-BY +02-30": false} {
		if got := validCalendar(cal); got != want {
			t.Errorf("validCalendar(%q) = %v", cal, got)
		}
	}

	munich := "DE-BY +08-15"
	if !isHoliday(munich, day(t, "2029-08-15")) || isHoliday("DE-BY", day(t, "2029-08-15")) {
		t.Error("15 August is a holiday in Munich, not in all of Bavaria")
	}
	if !isHoliday(munich, day(t, "2029-05-10")) {
		t.Error("the state's own holidays still count (Christi Himmelfahrt)")
	}
}
