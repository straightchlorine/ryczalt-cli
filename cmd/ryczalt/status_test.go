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
		json.NewEncoder(w).Encode(map[string]any{"rates": []map[string]any{
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
	failed := printStatus(out, c, 2026, today, months, nil)
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
  zus         1 286.76 PLN (social 456.18, health 830.58)
  ryczalt     not yet invoiced
  total       1 286.76 PLN
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
