package main

import (
	"strings"
	"testing"
	"time"
)

// The same whole year as TestSettleYear, worked out by hand there.
func TestForms(t *testing.T) {
	fakeNBPFlat(t)
	oldClock := clock
	clock = func() time.Time { return time.Date(2027, 3, 1, 12, 0, 0, 0, warsaw) }
	t.Cleanup(func() { clock = oldClock })

	var invs []invoice
	for m := 1; m <= 12; m++ {
		start := time.Date(2026, time.Month(m), 1, 0, 0, 0, 0, time.UTC)
		invs = append(invs, invoice{number: start.Format("INV-2006-01"), issued: start.AddDate(0, 1, 0),
			serviceEnd: start.AddDate(0, 1, -1), currency: "EUR", total: ratOf(333300)})
	}

	c := config{rate: "12", revenueBasis: "period_end", businessStart: day(t, "2026-01-01"),
		preferentialUntil: day(t, "2027-12-01"), chorobowa: true}
	today := day(t, "2027-03-01")

	months, _ := buildYear(c, invs, nil, 2026, today, t.TempDir())
	var pays []payment
	for _, m := range months {
		for kind, g := range map[string]int64{"social": m.social.grosze, "health": m.health.grosze, "ryczalt": m.ryczalt.grosze} {
			pays = append(pays, payment{paidOn: m.due, period: m.start, kind: kind, grosze: g})
		}
	}

	months, _ = buildYear(c, invs, pays, 2026, today, t.TempDir())
	w := newReport(nil)
	printForms(w, c, settleYear(c, months, pays, 2026, today), months, pays, today, false)

	out := w.b.String()
	for _, want := range []string{
		"P_30     revenue, own business, 12%            159 984.00",
		"P_98     social contributions paid               5 017.98",
		"P_105    health contributions (art. 11 1a)       3 903.73  50% of 7 807.46 paid",
		"P_202    tax base, 12%                         151 062.00",
		"P_232    ryczalt due                            18 127.00",
		"P_236    ryczalt paid                           19 198.00",
		"P_238    overpayment                             1 071.00",
		"23       annual base                           110 743.68  12 x 9 228.64",
		"24       annual contribution                     9 966.93  9% of field 23",
		"26       to pay                                  1 328.89",
		"P_239-50 ryczalt per month (part Q)",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("forms lack %q:\n%s", want, out)
		}
	}

	if strings.Contains(out, "~") {
		t.Error("a complete year shows no estimate")
	}
}

// Part Q is twelve fields, also when the year ends early because the
// business is suspended from November.
func TestFormsPartQTwelveMonths(t *testing.T) {
	fakeNBPFlat(t)
	c := config{rate: "12", revenueBasis: "period_end", businessStart: day(t, "2026-01-01"),
		preferentialUntil: day(t, "2027-12-01"), chorobowa: true, suspendedFrom: day(t, "2026-11-01")}
	today := day(t, "2026-11-25")

	var invs []invoice
	for m := 1; m <= 10; m++ {
		start := time.Date(2026, time.Month(m), 1, 0, 0, 0, 0, time.UTC)
		invs = append(invs, invoice{number: start.Format("INV-2006-01"), issued: start.AddDate(0, 1, 0),
			serviceEnd: start.AddDate(0, 1, -1), currency: "EUR", total: ratOf(333300)})
	}

	months, _ := buildYear(c, invs, nil, 2026, today, t.TempDir())
	s := settleYear(c, months, nil, 2026, today)
	if !s.complete {
		t.Fatal("the year is complete")
	}

	w := newReport(nil)
	printForms(w, c, s, months, nil, today, false)
	if want := "Nov 0, Dec 0"; !strings.Contains(w.b.String(), want) {
		t.Errorf("part Q lacks %q:\n%s", want, w.b.String())
	}
}
