package main

import (
	"math/big"
	"os"
	"testing"
	"time"
)

// Expected figures are the ones ZUS and biznes.gov.pl publish for 2026; the
// 2027 preferential amount follows from the 4950 PLN minimum wage.
func TestContributions(t *testing.T) {
	health := func(y, b int) amount { return healthMonthly(y, b) }
	social := func(y int, pref, chor bool) amount { s, _ := socialMonthly(y, pref, chor); return s }
	fpfs := func(y int, pref bool) amount { _, f := socialMonthly(y, pref, true); return f }

	cases := []struct {
		name string
		got  amount
		want string
	}{
		{"health 2026 low", health(2026, 0), "498.35"},
		{"health 2026 mid", health(2026, 1), "830.58"},
		{"health 2026 high", health(2026, 2), "1 495.04"},
		{"preferential 2026", social(2026, true, true), "456.18"},
		{"preferential 2026 without chorobowa", social(2026, true, false), "420.86"},
		{"full 2026", social(2026, false, true), "1 788.29"},
		{"full 2026 without chorobowa", social(2026, false, false), "1 649.82"},
		{"FP/FS 2026 full", fpfs(2026, false), "138.47"},
		{"FP/FS 2026 preferential", fpfs(2026, true), "0.00"},
		{"preferential 2027", social(2027, true, true), "469.85"},
		{"full 2027 is a forecast", social(2027, false, true), "~1 904.66"},
		{"health 2027 unknown", health(2027, 1), "n/a"},
		{"year without rules", social(2024, true, true), "n/a"},
	}

	for _, c := range cases {
		if got := plnText(c.got); got != c.want {
			t.Errorf("%s = %s, want %s", c.name, got, c.want)
		}
	}

	// 2026 full ZUS with chorobowa and FP/FS, as ZUS publishes it.
	if s, f := socialMonthly(2026, false, true); s.grosze+f.grosze != 192676 {
		t.Errorf("full 2026 total = %d grosze, want 192676", s.grosze+f.grosze)
	}
}

func TestRuleTable(t *testing.T) {
	for y, keys := range rules {
		for k, r := range keys {
			if _, ok := new(big.Rat).SetString(r.value); !ok || r.source == "" || r.status == unknown {
				t.Errorf("rules %d %s: want a decimal value, a source and a known status, got %+v", y, k, r)
			}
		}
	}
}

func TestStaleRules(t *testing.T) {
	if got := staleRules(time.Date(2026, 6, 1, 12, 0, 0, 0, warsaw)); got != nil {
		t.Errorf("2026: %v", got)
	}
	if got := staleRules(time.Date(2027, 1, 31, 12, 0, 0, 0, warsaw)); got != nil {
		t.Errorf("January is a grace month: %v", got)
	}
	if got := staleRules(time.Date(2027, 2, 1, 12, 0, 0, 0, warsaw)); len(got) != 2 {
		t.Errorf("February 2027 with a forecast and a missing figure: %v", got)
	}
}

// The yearly reminder runs only on request (RYCZALT_CHECK_FRESH=1), e.g.
// from a cron job, so it never blocks an unrelated release.
func TestRulesFresh(t *testing.T) {
	if os.Getenv("RYCZALT_CHECK_FRESH") == "" {
		t.Skip("set RYCZALT_CHECK_FRESH=1 to check this year's figures are in")
	}
	for _, s := range staleRules(time.Now().In(warsaw)) {
		t.Errorf("rules %s: enter the official figure (see the comment on rules)", s)
	}
}

func TestPLNText(t *testing.T) {
	for g, want := range map[int64]string{0: "0.00", 5: "0.05", 99999: "999.99", 100000: "1 000.00",
		123456789: "1 234 567.89", -5: "-0.05", -123456789: "-1 234 567.89"} {
		if got := plnText(amount{g, official}); got != want {
			t.Errorf("plnText(%d) = %q, want %q", g, got, want)
		}
	}
}

func TestGrosze(t *testing.T) {
	for in, want := range map[string]int64{"10.005": 1001, "10.004": 1000, "-10.005": -1001, "-10.004": -1000,
		"-0.005": -1, "-2.50": -250, "0": 0} {
		r, _ := new(big.Rat).SetString(in)
		if got := grosze(r); got != want {
			t.Errorf("grosze(%s) = %d, want %d", in, got, want)
		}
	}
}
