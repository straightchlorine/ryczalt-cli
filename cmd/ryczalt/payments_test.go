package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPaymentsRoundTrip(t *testing.T) {
	root := t.TempDir()
	p := func(id, on, period, kind string, g int64) payment {
		return payment{id: id, paidOn: day(t, on), period: day(t, period+"-01"), kind: kind, grosze: g}
	}

	if _, err := appendPayments(root, []payment{p("a", "2026-11-18", "2026-10", "social", 42086), p("b", "2026-11-18", "2026-10", "health", 83058)}); err != nil {
		t.Fatal(err)
	}

	if _, err := appendPayments(root, []payment{p("c", "2026-11-20", "2026-10", "ryczalt", 175800)}); err != nil {
		t.Fatal(err)
	}

	// a void cancels; a December payment lands in next year's file
	if _, err := appendPayments(root, []payment{{id: "d", paidOn: day(t, "2026-11-21"), period: day(t, "2026-10-01"), kind: "void", voids: "c"}}); err != nil {
		t.Fatal(err)
	}

	if _, err := appendPayments(root, []payment{p("e", "2027-01-20", "2026-12", "social", 42086)}); err != nil {
		t.Fatal(err)
	}

	ps, warns, err := readPayments(root)
	if err != nil || len(warns) > 0 {
		t.Fatal(err, warns)
	}

	var ids []string
	for _, x := range ps {
		ids = append(ids, x.id)
	}

	if strings.Join(ids, ",") != "a,b,e" {
		t.Errorf("got %v, want a,b,e", ids)
	}
	if sum, last, ok := paidFor(ps, day(t, "2026-10-01"), "health"); !ok || sum != 83058 || last != day(t, "2026-11-18") {
		t.Errorf("paidFor: %d %v %v", sum, last, ok)
	}
	if _, _, ok := paidFor(ps, day(t, "2026-10-01"), "ryczalt"); ok {
		t.Error("the voided ryczalt payment still counts")
	}

	files, _ := filepath.Glob(filepath.Join(root, "*", "payments.*.csv"))
	if len(files) != 2 {
		t.Errorf("want one file per year paid, got %v", files)
	}
	if st, _ := os.Stat(files[0]); st.Mode().Perm() != 0o600 {
		t.Errorf("mode %v", st.Mode().Perm())
	}
}

func TestReadPaymentsProblems(t *testing.T) {
	const head = paymentsHeader + "\n"
	cases := map[string]struct{ body, want string }{
		"bad amount":  {head + "x,2026-11-18,2026-10,social,-5,\n", `amount "-5" is not a positive number`},
		"bad kind":    {head + "x,2026-11-18,2026-10,vat,5,\n", `kind "vat"`},
		"bad date":    {head + "x,18.11.2026,2026-10,social,5,\n", "needs an id, a paid_on date"},
		"wrong head":  {"id,date\n", "first line must be"},
		"short field": {head + "x,2026-11-18\n", "wrong number of fields"},
	}

	// a damaged file is left out with a warning; the other files still count
	for name, c := range cases {
		root := writeArchive(t, map[string]string{"2026/payments.box.csv": c.body,
			"2026/payments.other.csv": head + "y,2026-11-18,2026-10,social,5,\n"})
		ps, warns, err := readPayments(root)
		if err != nil || len(ps) != 1 || len(warns) != 1 || !strings.Contains(warns[0], c.want) {
			t.Errorf("%s: got %d payments, %q, %v; want a warning with %q", name, len(ps), warns, err, c.want)
		}
	}

	// a Syncthing conflict copy and a duplicate id are warnings, not silent
	root := writeArchive(t, map[string]string{
		"2026/payments.a.csv": head + "x,2026-11-18,2026-10,social,5,\n",
		"2026/payments.b.csv": head + "x,2026-11-18,2026-10,social,5,\n",
		"2026/payments.a.sync-conflict-20261118-120000-ABCDEFG.csv": head,
	})

	ps, warns, err := readPayments(root)
	if err != nil || len(ps) != 1 || len(warns) != 2 {
		t.Errorf("got %d payments, warnings %q, %v", len(ps), warns, err)
	}
}

func TestMonthStateWithPayments(t *testing.T) {
	m := month{
		start: day(t, "2026-10-01"), invoiced: true, due: day(t, "2026-11-20"),
		social: amount{42086, official}, health: amount{83058, official}, fpfs: amount{0, official},
		ryczalt: amount{175800, official},
	}

	paid := m
	paid.zusPaid, paid.zusPaidOn = 125144, day(t, "2026-11-18")
	paid.ryczPaid, paid.ryczPaidOn = true, day(t, "2026-11-20")
	lateRycz := paid
	lateRycz.ryczPaidOn = day(t, "2026-11-24")

	partial := m
	partial.zusPaid = 42086 // social only
	partial.ryczPaid, partial.ryczPaidOn = true, day(t, "2026-11-20")
	zusOnly := m
	zusOnly.zusPaid, zusOnly.zusPaidOn = 125144, day(t, "2026-11-18")

	cases := []struct {
		name        string
		m           month
		today       string
		text, level string
	}{
		{"nothing paid, past due", m, "2026-11-21", "zus, ryczalt", "FAIL"},
		{"all paid on time", paid, "2026-11-25", "", "OK"},
		{"ryczalt paid late", lateRycz, "2026-11-25", "late, 2026-11-24", "WARN"},
		{"health missing", partial, "2026-11-21", "zus", "FAIL"},
		{"zus paid early", zusOnly, "2026-11-19", "due tomorrow, zus paid", "WARN"},
		{"all paid before due", paid, "2026-11-20", "", "OK"},
	}

	for _, c := range cases {
		if text, level := monthState(c.m, day(t, c.today)); text != c.text || level != c.level {
			t.Errorf("%s: got %q %q, want %q %q", c.name, text, level, c.text, c.level)
		}
	}
}

func TestAppendAfterOpenLastLine(t *testing.T) {
	root := writeArchive(t, map[string]string{"2026/payments.box.csv": paymentsHeader + "\na,2026-11-18,2026-10,social,420.86,"})
	host, _ := os.Hostname()
	if err := os.Rename(filepath.Join(root, "2026", "payments.box.csv"), filepath.Join(root, "2026", "payments."+safeHost(host)+".csv")); err != nil {
		t.Fatal(err)
	}

	if _, err := appendPayments(root, []payment{{id: "b", paidOn: day(t, "2026-11-18"), period: day(t, "2026-10-01"), kind: "health", grosze: 83058}}); err != nil {
		t.Fatal(err)
	}

	ps, warns, err := readPayments(root)
	if err != nil || len(warns) > 0 || len(ps) != 2 {
		t.Errorf("got %d payments, %q, %v", len(ps), warns, err)
	}
}

func TestParseAmount(t *testing.T) {
	for in, want := range map[string]int64{"1758": 175800, "830.58": 83058, "830,58": 83058, "0.5": 50, "0": 0,
		"1e3": -1, "1/3": -1, "0x10": -1, "1_000": -1, "-5": -1, "12.345": -1, "9999999999": -1, "": -1} {
		if got := parseAmount(in); got != want {
			t.Errorf("parseAmount(%q) = %d, want %d", in, got, want)
		}
	}
}
