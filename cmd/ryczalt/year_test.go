package main

import (
	"strings"
	"testing"
	"time"
)

// A whole 2026 at 13 332.00 PLN a month (3333 EUR at 4.0000), preferential
// ZUS with chorobowa, every month paid on its deadline. By hand:
//   - revenue 12 x 13 332.00 = 159 984.00
//   - social paid in 2026: January..November's, 11 x 456.18 = 5 017.98
//     (December's is paid in January 2027)
//   - health paid in 2026: 4 x 498.35 + 7 x 830.58 = 7 807.46, half 3 903.73
//   - base 159 984.00 - 8 921.71 = 151 062.29 -> 151 062; 12% = 18 127.44 -> 18 127
//   - advances: all twelve, 19 198 (12% of the whole year's revenue, month by month)
//   - refund 19 198 - 18 127 = 1 071
//   - health: 159 984.00 - 5 017.98 = 154 966.02, bracket 60k-300k:
//     9% of 12 x 9 228.64 = 9 966.93 against 8 638.04 due monthly, top-up 1 328.89
func TestSettleYear(t *testing.T) {
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

	months, notes := buildYear(c, invs, nil, 2026, today, t.TempDir())
	if len(notes) > 0 {
		t.Fatal(notes)
	}

	var pays []payment
	for _, m := range months {
		for kind, g := range map[string]int64{"social": m.social.grosze, "health": m.health.grosze, "ryczalt": m.ryczalt.grosze} {
			pays = append(pays, payment{id: m.start.Format("2006-01") + kind, paidOn: m.due, period: m.start, kind: kind, grosze: g})
		}
	}

	months, _ = buildYear(c, invs, pays, 2026, today, t.TempDir()) // now with the payments
	s := settleYear(c, months, pays, 2026, today)

	got := map[string]int64{"revenue": s.revenue.grosze, "social": s.socialPaid, "health": s.healthPaid,
		"base": s.base, "tax": s.tax, "advances": s.advances, "result": s.result,
		"year health": s.healthYear.grosze, "monthly health": s.healthMonthly, "settle": s.healthSettle}
	want := map[string]int64{"revenue": 15998400, "social": 501798, "health": 780746,
		"base": 15106200, "tax": 1812700, "advances": 1919800, "result": -107100,
		"year health": 996693, "monthly health": 863804, "settle": 132889}

	for k, w := range want {
		if got[k] != w {
			t.Errorf("%s = %d, want %d", k, got[k], w)
		}
	}

	if !s.complete || s.months != 12 || s.bracket != 1 {
		t.Errorf("complete=%v months=%d bracket=%d", s.complete, s.months, s.bracket)
	}

	w := newReport(nil)
	printYear(w, c, s, months, pays, today)
	out := w.b.String()
	if strings.Contains(out, "~") || strings.Contains(out, "estimate") {
		t.Errorf("a complete year shows no estimate:\n%s", out)
	}

	for _, want := range []string{
		"  invoiced                  Jan-Dec      12 of 12 months",
		"  ZUS paid                  Jan-Dec      12 of 12 months",
		"    - social ZUS paid           -5 017.98  for Jan-Nov, paid in 2026",
		"    - 50% of health ZUS paid    -3 903.73  half of 7 807.46: ryczalt deducts half",
		"    = refund                     1 071.00",
		"    - billed month by month     -8 638.04  Jan-Apr 498.35, May-Dec 830.58",
		"    = to pay by 20 May           1 328.89  Jan-Apr billed at 498.35 instead of 830.58",
		"PIT-28 for 2026  filed 2027-02-15 to 2027-04-30",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("output lacks %q:\n%s", want, out)
		}
	}
}

// Deductions larger than revenue: the rest is lost, the base is zero.
func TestSettleYearDeductionsCapped(t *testing.T) {
	c := config{rate: "12"}
	months := []month{{start: day(t, "2026-01-01"), revenue: amount{10000, official}, invoiced: true}}
	pays := []payment{{paidOn: day(t, "2026-01-20"), period: day(t, "2025-12-01"), kind: "social", grosze: 50000}}
	if s := settleYear(c, months, pays, 2026, day(t, "2027-03-01")); s.deductions != 10000 || s.base != 0 || s.tax != 0 {
		t.Errorf("deductions %d base %d tax %d", s.deductions, s.base, s.tax)
	}
}

func TestMonthRanges(t *testing.T) {
	m := func(s string) time.Time { return day(t, s+"-01") }
	for want, ms := range map[string][]time.Time{
		"Jan-Oct":           {m("2026-01"), m("2026-02"), m("2026-03"), m("2026-04"), m("2026-05"), m("2026-06"), m("2026-07"), m("2026-08"), m("2026-09"), m("2026-10")},
		"Jan-Mar, May":      {m("2026-01"), m("2026-02"), m("2026-03"), m("2026-05")},
		"Dec 2025-Oct 2026": {m("2025-12"), m("2026-01"), m("2026-02"), m("2026-03"), m("2026-04"), m("2026-05"), m("2026-06"), m("2026-07"), m("2026-08"), m("2026-09"), m("2026-10")},
		"Dec 2025, Feb":     {m("2025-12"), m("2026-02")},
		"none":              nil,
	} {
		if got := monthRanges(ms, 2026); got != want {
			t.Errorf("got %q, want %q", got, want)
		}
	}
}

// Ten months in, every one paid: the estimate assumes the last two go
// like October and are paid in full on their deadlines.
func TestYearEstimate(t *testing.T) {
	fakeNBPFlat(t)
	oldClock := clock
	clock = func() time.Time { return time.Date(2026, 11, 25, 12, 0, 0, 0, warsaw) }
	t.Cleanup(func() { clock = oldClock })

	var invs []invoice
	for m := 1; m <= 10; m++ {
		start := time.Date(2026, time.Month(m), 1, 0, 0, 0, 0, time.UTC)
		invs = append(invs, invoice{number: start.Format("INV-2006-01"), issued: start.AddDate(0, 1, 0),
			serviceEnd: start.AddDate(0, 1, -1), currency: "EUR", total: ratOf(333300)})
	}

	c := config{rate: "12", revenueBasis: "period_end", businessStart: day(t, "2026-01-01"),
		preferentialUntil: day(t, "2027-12-01"), chorobowa: true}
	months, _ := buildYear(c, invs, nil, 2026, day(t, "2026-11-25"), t.TempDir())

	var pays []payment
	for _, m := range months[:10] {
		for kind, g := range map[string]int64{"social": m.social.grosze, "health": m.health.grosze, "ryczalt": m.ryczalt.grosze} {
			pays = append(pays, payment{paidOn: m.due, period: m.start, kind: kind, grosze: g})
		}
	}

	ext, extPays, ok := fullYear(c, months, pays, 2026)
	if !ok || len(ext) != 12 {
		t.Fatalf("got %d months", len(ext))
	}

	// the same whole year as TestSettleYear, so the same results
	est := settleYear(c, ext, extPays, 2026, day(t, "2026-12-31"))
	if est.result != -107100 || est.healthSettle != 132889 || est.months != 12 {
		t.Errorf("estimate: result %d, health %d, months %d", est.result, est.healthSettle, est.months)
	}
}

func TestHealthCliff(t *testing.T) {
	fakeNBPFlat(t)
	oldClock := clock
	clock = func() time.Time { return time.Date(2026, 11, 25, 12, 0, 0, 0, warsaw) }
	t.Cleanup(func() { clock = oldClock })

	c := config{rate: "12", revenueBasis: "period_end", businessStart: day(t, "2026-01-01"),
		preferentialUntil: day(t, "2027-12-01"), chorobowa: true}
	today := day(t, "2026-11-25")

	cliff := func(eurCents int64) (finding, bool) {
		var invs []invoice
		for m := 1; m <= 10; m++ {
			start := time.Date(2026, time.Month(m), 1, 0, 0, 0, 0, time.UTC)
			invs = append(invs, invoice{number: start.Format("INV-2006-01"), issued: start.AddDate(0, 1, 0),
				serviceEnd: start.AddDate(0, 1, -1), currency: "EUR", total: ratOf(eurCents)})
		}

		months, _ := buildYear(c, invs, nil, 2026, today, t.TempDir())
		return healthCliff(c, months, nil, 2026, today)
	}

	// 3333 EUR a month: ~155k for the year, far from both lines
	if f, ok := cliff(333300); ok {
		t.Errorf("no warning expected, got %q", f.text)
	}

	// 6175 EUR a month (24 700 PLN): ~291k for the year, a month short of 300k
	if f, ok := cliff(617500); !ok || !strings.Contains(f.text, "below the 300k line") {
		t.Errorf("want a warning below 300k, got %q %v", f.text, ok)
	}

	// 6400 EUR a month (25 600 PLN): ~302k, just over
	if f, ok := cliff(640000); !ok || !strings.Contains(f.text, "only") || !strings.Contains(f.text, "above the 300k line") {
		t.Errorf("want a warning just above 300k, got %q %v", f.text, ok)
	}
}
