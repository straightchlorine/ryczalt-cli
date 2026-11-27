package main

import (
	"strings"
	"testing"
	"time"
)

// Suspended from February: nothing is due for it, nothing is expected, and
// the year's health settlement counts January only.
func TestSuspension(t *testing.T) {
	fakeNBPFlat(t)
	oldClock := clock
	clock = func() time.Time { return time.Date(2026, 5, 25, 12, 0, 0, 0, warsaw) }
	t.Cleanup(func() { clock = oldClock })

	c := config{rate: "12", revenueBasis: "period_end", businessStart: day(t, "2026-01-01"),
		preferentialUntil: day(t, "2027-12-01"), chorobowa: true, suspendedFrom: day(t, "2026-02-01")}
	jan := invoice{number: "INV-01", issued: day(t, "2026-02-02"), serviceEnd: day(t, "2026-01-31"),
		currency: "EUR", total: ratOf(333300)}

	today := day(t, "2026-05-25")
	months, notes := buildYear(c, []invoice{jan}, nil, 2026, today, t.TempDir())
	if len(notes) > 0 {
		t.Fatal(notes)
	}

	for _, m := range months[1:] {
		if !m.suspended || m.zusDue().grosze != 0 || m.zusOpen() || m.ryczOwed() {
			t.Errorf("%s: want suspended with nothing due, got %+v", m.start.Format("2006-01"), m)
		}

		if text, level := monthState(m, today); text != "suspended" || level != "" {
			t.Errorf("%s: state %q %q", m.start.Format("2006-01"), text, level)
		}
	}

	s := settleYear(c, months, nil, 2026, today)
	if s.months != 1 || s.healthMonthly != months[0].health.grosze {
		t.Errorf("health settlement counts %d months", s.months)
	}

	// a suspension that ends: from June the business runs again
	c.suspendedUntil = day(t, "2026-04-01")
	months, _ = buildYear(c, []invoice{jan}, nil, 2026, today, t.TempDir())
	if months[4].suspended || months[4].zusDue().grosze == 0 {
		t.Errorf("May is not suspended any more: %+v", months[4])
	}

	// suspended all year: no settlement at all
	c = config{rate: "12", businessStart: day(t, "2025-06-01"), suspendedFrom: day(t, "2026-01-01")}
	months, _ = buildYear(c, nil, nil, 2026, today, t.TempDir())

	w := newReport(nil)
	printYear(w, c, settleYear(c, months, nil, 2026, today), months, nil, today)
	if !strings.Contains(w.b.String(), "no insured month in the year: no settlement") {
		t.Errorf("whole year suspended:\n%s", w.b.String())
	}
}

func TestSuspensionConfig(t *testing.T) {
	for _, c := range []struct{ conf, want string }{
		{"suspended_until=2027-06", "tax.suspended_until: needs tax.suspended_from"},
		{"suspended_from=2027-06\nsuspended_until=2027-01", "tax.suspended_until: before tax.suspended_from"},
		{"suspended_from=2025-01", "tax.suspended_from: before tax.business_start"},
	} {
		in := strings.Replace(validConfig, "business_start=2025-06", "business_start=2025-06\n"+c.conf, 1)
		if _, err := parseConfig(strings.NewReader(in)); err == nil || !strings.Contains(err.Error(), c.want) {
			t.Errorf("%s: got %v", c.conf, err)
		}
	}

	in := strings.Replace(validConfig, "business_start=2025-06", "business_start=2025-06\nsuspended_from=2027-01", 1)
	c, err := parseConfig(strings.NewReader(in))
	if err != nil || !c.suspended(day(t, "2030-05-01")) || c.suspended(day(t, "2026-12-01")) {
		t.Errorf("open-ended suspension: %v %+v", err, c.suspendedFrom)
	}
}

// The social contributions of a suspended month are not paid, so they do not
// lower the base of the health bracket: 60 500 revenue in July less July's
// social alone is over the 60 000 line.
func TestSuspensionHealthBracket(t *testing.T) {
	fakeNBPFlat(t)
	c := config{rate: "12", revenueBasis: "period_end", businessStart: day(t, "2026-01-01"),
		preferentialUntil: day(t, "2027-12-01"), chorobowa: true,
		suspendedFrom: day(t, "2026-01-01"), suspendedUntil: day(t, "2026-06-01")}

	jul := invoice{number: "INV-07", issued: day(t, "2026-08-02"), serviceEnd: day(t, "2026-07-31"),
		currency: "EUR", total: ratOf(1512500)}
	months, _ := buildYear(c, []invoice{jul}, nil, 2026, day(t, "2026-09-25"), t.TempDir())
	if months[6].bracket != 1 {
		t.Errorf("July: bracket %d, want 1", months[6].bracket)
	}
}
