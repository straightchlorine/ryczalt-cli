package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"
)

// fakeNBPFlat answers every EUR lookup with 4.0000, published on the last
// day asked for.
func fakeNBPFlat(t *testing.T) {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/") // eur/FROM/TO
		_ = json.NewEncoder(w).Encode(map[string]any{"rates": []map[string]any{
			{"no": "001/A/NBP/2026", "effectiveDate": parts[2], "mid": 4.0},
		}})
	}))

	old := nbpURL
	nbpURL = srv.URL
	t.Cleanup(func() { nbpURL = old; srv.Close() })
}

// Five months invoiced at 3333 EUR (13 332.00 PLN each), June not invoiced
// by 15 July, July in progress; payments as below. Worked out by hand:
//   - ryczalt from 1 January: 1600 (1599.84), 3200, 4800, 6399, 7999 in total
//   - health: revenue less social passes 60 000 in May (66 660 - 5 x 456.18)
//   - 20 June 2026 is a Saturday, so May is due on Monday 22 June
func TestStatus(t *testing.T) {
	fakeNBPFlat(t)
	files := map[string]string{}
	for _, m := range []string{"02", "03", "04", "05", "06"} {
		start := "2026-" + m + "-01"
		end := day(t, start).AddDate(0, 0, -1).Format(time.DateOnly)
		files["2026-"+m+"/ksef-INV-"+m+".xml"] = fa("INV-"+m, start, end)
	}

	c := config{
		archive: writeArchive(t, files), nip: testNIP, rate: "12", revenueBasis: "period_end",
		businessStart: day(t, "2026-01-01"), preferentialUntil: day(t, "2027-12-01"), chorobowa: true,
	}
	today := day(t, "2026-07-20")

	invs, found, err := readArchive(c.archive, c.nip)
	if err != nil || len(found) > 0 {
		t.Fatal(err, found)
	}

	// January to April paid on their deadlines, May's ryczalt late, June's
	// ZUS early (June has no invoice yet), nothing for July
	var pays []payment
	pay := func(on, period, kind string, g int64) {
		pays = append(pays, payment{id: on + kind, paidOn: day(t, on), period: day(t, period+"-01"), kind: kind, grosze: g})
	}

	for i, due := range []string{"2026-02-20", "2026-03-20", "2026-04-20", "2026-05-20"} {
		period := fmt.Sprintf("2026-%02d", i+1)
		pay(due, period, "social", 45618)
		pay(due, period, "health", 49835)
		pay(due, period, "ryczalt", 160000)
	}

	pay("2026-06-22", "2026-05", "social", 45618)
	pay("2026-06-22", "2026-05", "health", 83058)
	pay("2026-06-25", "2026-05", "ryczalt", 160000)
	pay("2026-07-10", "2026-06", "social", 45618)
	pay("2026-07-10", "2026-06", "health", 83058)

	months, notes := buildYear(c, invs, pays, 2026, today, t.TempDir())
	if len(notes) > 0 {
		t.Fatal(notes)
	}

	out, err := os.CreateTemp(t.TempDir(), "status")
	if err != nil {
		t.Fatal(err)
	}

	r := newReport(out)
	failed := printStatus(r, c, 2026, today, months, nil)
	if err := r.flush(); err != nil {
		t.Fatal(err)
	}

	b, _ := os.ReadFile(out.Name())

	want := `ryczalt 2026
Mon 2026-07-20 | 12% ryczalt | preferential ZUS with chorobowa | rules official

Month    Invoice  Revenue PLN  Ryczalt*  Social  Health  Due             State
2026-01  INV-02     13 332.00     1 600  456.18  498.35  Fri 2026-02-20  [  OK  ]
2026-02  INV-03     13 332.00     1 600  456.18  498.35  Fri 2026-03-20  [  OK  ]
2026-03  INV-04     13 332.00     1 600  456.18  498.35  Mon 2026-04-20  [  OK  ]
2026-04  INV-05     13 332.00     1 599  456.18  498.35  Wed 2026-05-20  [  OK  ]
2026-05  INV-06     13 332.00     1 600  456.18  830.58  Mon 2026-06-22  [ WARN ] late, 2026-06-25
2026-06  -                  -       n/a  456.18  830.58  Mon 2026-07-20  [ FAIL ] no invoice; due by 2026-07-15
2026-07  -                  -       n/a  456.18  830.58  Thu 2026-08-20  in progress

* in full, before deducting paid ZUS (social, half of health); PIT-28 settles the deductions

Year so far   66 660.00 PLN revenue, health bracket 60k-300k

Next payment  Thu 2026-08-20, for 2026-07
  zus             1 286.76  social 456.18, health 830.58
  ryczalt                   not yet invoiced
  total           1 286.76  PLN
`
	if string(b) != want {
		t.Errorf("got\n%s\nwant\n%s", b, want)
	}
	if !failed {
		t.Error("a month without an invoice past the 15th must fail")
	}
}

func TestMonthState(t *testing.T) {
	m := month{start: day(t, "2026-10-01"), invoiced: true, due: day(t, "2026-11-20")}
	cases := []struct{ today, text, level string }{
		{"2026-10-31", "in progress", ""},
		{"2026-11-01", "due in 19 days", ""},
		{"2026-11-13", "due in 7 days", "WARN"},
		{"2026-11-20", "due today", "WARN"},
		{"2026-11-21", "zus, ryczalt", "FAIL"},
	}

	for _, c := range cases {
		if text, level := monthState(m, day(t, c.today)); text != c.text || level != c.level {
			t.Errorf("%s: got %q %q, want %q %q", c.today, text, level, c.text, c.level)
		}
	}

	m.invoiced = false
	if text, level := monthState(m, day(t, "2026-11-15")); level != "INFO" || !strings.Contains(text, "2026-11-16") {
		t.Errorf("15 Nov 2026 is a Sunday, so the invoice is due on Monday the 16th: got %q %q", text, level)
	}
}

func TestZusPhase(t *testing.T) {
	c := config{ulgaUntil: day(t, "2025-11-01"), preferentialUntil: day(t, "2027-11-01")}
	for m, want := range map[string]string{"2025-11-01": "ulga", "2025-12-01": "preferential", "2027-11-01": "preferential", "2027-12-01": "full"} {
		if got := zusPhase(c, day(t, m)); got != want {
			t.Errorf("%s: got %s, want %s", m, got, want)
		}
	}

	if got := zusPhase(config{}, day(t, "2026-01-01")); got != "full" {
		t.Errorf("no relief configured: got %s", got)
	}
}

func TestWholePLN(t *testing.T) {
	for g, want := range map[int64]int64{149: 100, 150: 200, 175802: 175800, 175850: 175900, -150: -200} {
		if got := wholePLN(g); got != want {
			t.Errorf("wholePLN(%d) = %d, want %d", g, got, want)
		}
	}
}

// Suspended months have nothing due, so the hint to record payments starts
// at March, the first month with something to pay.
func TestStatusHintSkipsSuspended(t *testing.T) {
	c := config{rate: "12", revenueBasis: "period_end", businessStart: day(t, "2026-01-01"),
		preferentialUntil: day(t, "2027-12-01"), chorobowa: true,
		suspendedFrom: day(t, "2026-01-01"), suspendedUntil: day(t, "2026-02-01")}

	today := day(t, "2026-06-10")
	months, _ := buildYear(c, nil, nil, 2026, today, t.TempDir())

	r := newReport(nil)
	printStatus(r, c, 2026, today, months, nil)
	if want := "ryczalt paid --on-due 2026-03..2026-04\n"; !strings.Contains(r.b.String(), want) {
		t.Errorf("want %q in:\n%s", want, r.b.String())
	}
}

// A business that began in November 2026, with invoices for November and
// December, as the status view of today shows it: this year's months with
// last year's open ones in front, as cmdStatus builds them.
func carryView(t *testing.T, c config, files map[string]string, pays []payment, today string) (string, bool) {
	t.Helper()
	fakeNBPFlat(t)

	c.archive, c.nip, c.rate, c.revenueBasis = writeArchive(t, files), testNIP, "12", "period_end"
	c.preferentialUntil, c.chorobowa = day(t, "2027-12-01"), true

	now := day(t, today)
	oldClock := clock
	clock = func() time.Time { return now.Add(12 * time.Hour) }
	t.Cleanup(func() { clock = oldClock })

	invs, _, err := readArchive(c.archive, c.nip)
	if err != nil {
		t.Fatal(err)
	}

	months, _ := buildYear(c, invs, pays, now.Year(), now, t.TempDir())
	carried, _ := carriedOver(c, invs, pays, now.Year(), now, t.TempDir())

	r := newReport(nil)
	failed := printStatus(r, c, now.Year(), now, append(carried, months...), nil)

	return r.b.String(), failed
}

func novDecFiles() map[string]string {
	return map[string]string{
		"2026-11/ksef-INV-11.xml": fa("INV-11", "2026-12-01", "2026-11-30"),
		"2026-12/ksef-INV-12.xml": fa("INV-12", "2027-01-01", "2026-12-31"),
	}
}

func paidMonth(t *testing.T, period string) []payment {
	on, p := day(t, "2026-12-15"), day(t, period+"-01")
	return []payment{
		{id: period + "s", paidOn: on, period: p, kind: "social", grosze: 45618},
		{id: period + "h", paidOn: on, period: p, kind: "health", grosze: 49835},
		{id: period + "r", paidOn: on, period: p, kind: "ryczalt", grosze: 160000},
	}
}

func TestStatusCarriesLastYear(t *testing.T) {
	c := config{businessStart: day(t, "2026-11-01")}
	pays := paidMonth(t, "2026-11")

	out, failed := carryView(t, c, novDecFiles(), pays, "2027-01-10")
	if !strings.Contains(out, "\n2026-12 ") || strings.Contains(out, "2026-11") {
		t.Errorf("want December in the table and November out of it:\n%s", out)
	}
	if !strings.Contains(out, "Next payment  Wed 2027-01-20, for 2026-12\n") || failed {
		t.Errorf("failed %v, want December as the next payment:\n%s", failed, out)
	}

	out, failed = carryView(t, c, novDecFiles(), pays, "2027-01-25")
	if !strings.Contains(out, "Overdue       Wed 2027-01-20, for 2026-12\n") || !failed {
		t.Errorf("failed %v, want December overdue:\n%s", failed, out)
	}

	out, failed = carryView(t, c, novDecFiles(), append(pays, paidMonth(t, "2026-12")...), "2027-01-25")
	if strings.Contains(out, "\n2026-1") || failed {
		t.Errorf("failed %v, want no 2026 row once December is paid:\n%s", failed, out)
	}
}

func TestStatusYearSoFarIsThisYear(t *testing.T) {
	files := novDecFiles()
	files["2027-01/ksef-INV-01.xml"] = fa("INV-01", "2027-02-01", "2027-01-31")

	c := config{businessStart: day(t, "2026-11-01")}
	out, _ := carryView(t, c, files, paidMonth(t, "2026-11"), "2027-02-05")

	if !strings.Contains(out, "\n2026-12 ") || !strings.Contains(out, "Year so far   13 332.00 PLN revenue, health bracket up to 60k\n") {
		t.Errorf("want December listed and 2027 revenue only in the year so far:\n%s", out)
	}
}

func TestStatusSettleLine(t *testing.T) {
	both := "[ INFO ] 2026 still to settle: PIT-28 between 2027-02-15 and 2027-04-30, health settlement by 2027-05-20 (ryczalt year)\n"
	health := "[ INFO ] 2026 still to settle: health settlement by 2027-05-20 (ryczalt year)\n"
	c := config{businessStart: day(t, "2026-11-01")}

	for today, want := range map[string]string{"2027-01-10": both, "2027-05-10": health, "2027-06-01": ""} {
		out, _ := carryView(t, c, novDecFiles(), nil, today)
		if want == "" && strings.Contains(out, "still to settle") || !strings.Contains(out, want) {
			t.Errorf("%s: want %q in:\n%s", today, want, out)
		}
	}

	c.suspendedFrom = day(t, "2026-11-01")
	if out, _ := carryView(t, c, nil, nil, "2027-01-10"); strings.Contains(out, "still to settle") {
		t.Errorf("last year was suspended throughout:\n%s", out)
	}
}

// The hint to record payments speaks for this year's months. Carried months
// are the unpaid ones of a year that has payments, so they neither trigger
// it nor enter its range.
func TestStatusHintIsForThisYear(t *testing.T) {
	c := config{businessStart: day(t, "2026-11-01")}

	// January: November and December are carried and overdue, nothing of 2027 is due yet
	out, _ := carryView(t, c, nil, nil, "2027-01-25")
	if !strings.Contains(out, "2026-12") || strings.Contains(out, "no payments recorded yet") {
		t.Errorf("carried months alone must not bring the hint:\n%s", out)
	}

	// March: January and February of 2027 are due and unpaid
	out, _ = carryView(t, c, nil, nil, "2027-03-25")
	if !strings.Contains(out, "ryczalt paid --on-due 2027-01..2027-02\n") {
		t.Errorf("want the hint for the months of 2027 only:\n%s", out)
	}
}

// Outside the window the view is what it always was.
func TestStatusOutsideWindowUnchanged(t *testing.T) {
	c := config{businessStart: day(t, "2026-10-01")}
	files := map[string]string{"2026-10/ksef-INV-10.xml": fa("INV-10", "2026-11-01", "2026-10-31")}

	out, failed := carryView(t, c, files, nil, "2026-11-25")
	want := `ryczalt 2026
Wed 2026-11-25 | 12% ryczalt | preferential ZUS with chorobowa | rules official

Month    Invoice  Revenue PLN  Ryczalt*  Social  Health  Due             State
2026-10  INV-10     13 332.00     1 600  456.18  498.35  Fri 2026-11-20  [ FAIL ] zus, ryczalt
2026-11  -                  -       n/a  456.18  498.35  Mon 2026-12-21  in progress

* in full, before deducting paid ZUS (social, half of health); PIT-28 settles the deductions

Year so far   13 332.00 PLN revenue, health bracket up to 60k

Overdue       Fri 2026-11-20, for 2026-10
  zus               954.53  social 456.18, health 498.35
  ryczalt         1 600.00  in full*
  total           2 554.53  PLN

[ INFO ] no payments recorded yet; if all were paid on time: ryczalt paid --on-due 2026-10
`
	if out != want || !failed {
		t.Errorf("failed %v:\n%s", failed, out)
	}
}
