package main

import (
	"slices"
	"strings"
	"time"
)

// deStates are the German lands, as ISO 3166-2 codes without "DE-".
var deStates = []string{
	"BB", "BE", "BW", "BY", "HB", "HE", "HH", "MV",
	"NI", "NW", "RP", "SH", "SL", "SN", "ST", "TH",
}

// validCalendar reports whether cal names a holiday calendar: "PL", or
// "DE-" plus a state, e.g. "DE-BY".
func validCalendar(cal string) bool {
	state, ok := strings.CutPrefix(cal, "DE-")
	return cal == "PL" || ok && slices.Contains(deStates, state)
}

// easter returns Easter Sunday of the Gregorian calendar, as midnight UTC.
func easter(y int) time.Time {
	a := y % 19
	b, c := y/100, y%100
	d, e := b/4, b%4
	f := (b + 8) / 25
	g := (b - f + 1) / 3
	h := (19*a + b - d - g + 15) % 30
	i, k := c/4, c%4
	l := (32 + 2*e + 2*i - h - k) % 7
	m := (a + 11*h + 22*l) / 451
	n := h + l - 7*m + 114
	return time.Date(y, time.Month(n/31), n%31+1, 0, 0, 0, 0, time.UTC)
}

// isHoliday reports a public holiday in calendar cal (see validCalendar).
func isHoliday(cal string, t time.Time) bool {
	if cal == "PL" {
		return isHolidayPL(t)
	}
	state, ok := strings.CutPrefix(cal, "DE-")
	return ok && isHolidayDE(state, t)
}

// isHolidayPL - holiday days of Poland (from the ustawa o dniach wolnych od pracy)
func isHolidayPL(t time.Time) bool {
	y, m, d := t.Date()
	switch e := t.YearDay() - easter(y).YearDay(); {
	case m == time.December && d == 24: // Wigilia Bożego Narodzenia
		return y >= 2025
	case m == time.January && d == 1, // Nowy Rok
		m == time.January && d == 6,   // Święto Trzech Króli
		e == 0,                        // Wielkanoc
		e == 1,                        // Poniedziałek Wielkanocny
		m == time.May && d == 1,       // Święto Pracy
		m == time.May && d == 3,       // Święto Konstytucji 3 Maja
		e == 49,                       // Zielone Świątki
		e == 60,                       // Boże Ciało
		m == time.August && d == 15,   // Wniebowzięcie Najświętszej Maryi Panny
		m == time.November && d == 1,  // Wszystkich Świętych
		m == time.November && d == 11, // Narodowe Święto Niepodległości
		m == time.December && d == 25, // pierwszy dzień Bożego Narodzenia
		m == time.December && d == 26: // drugi dzień Bożego Narodzenia
		return true
	}
	return false
}

// isHolidayDE covers the holidays a whole state keeps and adds on top of it
// holidays kept only in some lands.
func isHolidayDE(state string, t time.Time) bool {
	y, m, d := t.Date()
	in := func(states string) bool { return slices.Contains(strings.Fields(states), state) }
	switch e := t.YearDay() - easter(y).YearDay(); {
	case m == time.January && d == 1, // Neujahr
		e == -2,                       // Karfreitag
		e == 1,                        // Ostermontag
		m == time.May && d == 1,       // Tag der Arbeit
		e == 39,                       // Christi Himmelfahrt
		e == 50,                       // Pfingstmontag
		m == time.October && d == 3,   // Tag der Deutschen Einheit
		m == time.December && d == 25, // 1. Weihnachtstag
		m == time.December && d == 26: // 2. Weihnachtstag
		return true
	case e == 0, e == 49: // Ostersonntag, Pfingstsonntag
		return in("BB")
	case e == 60: // Fronleichnam
		return in("BW BY HE NW RP SL")
	case m == time.January && d == 6: // Heilige Drei Könige
		return in("BW BY ST")
	case m == time.March && d == 8: // Internationaler Frauentag
		return in("BE MV")
	case m == time.August && d == 15: // Mariä Himmelfahrt
		return in("SL")
	case m == time.September && d == 20: // Weltkindertag
		return in("TH")
	case m == time.October && d == 31: // Reformationstag
		return in("BB HB HH MV NI SH SN ST TH")
	case m == time.November && d == 1: // Allerheiligen
		return in("BW BY NW RP SL")
	case m == time.November && d >= 16 && d <= 22 && t.Weekday() == time.Wednesday: // Buß- und Bettag
		return in("SN")
	}
	return false
}

// isWorkingDay reports a day that is neither a weekend nor a holiday.
func isWorkingDay(cal string, t time.Time) bool {
	return t.Weekday() != time.Saturday && t.Weekday() != time.Sunday && !isHoliday(cal, t)
}

// shiftDeadline moves a deadline that falls on a Saturday, a Sunday or a
// holiday to the next working day.
//
// Compliant with:
// Polish tax law (Ordynacja podatkowa art. 12 § 5) and
// Polish civil law (KC art. 115)
// German civil law (BGB § 193)
//
// So there is no difference in handling this topic.
func shiftDeadline(cal string, t time.Time) time.Time {
	for !isWorkingDay(cal, t) {
		t = t.AddDate(0, 0, 1)
	}
	return t
}
