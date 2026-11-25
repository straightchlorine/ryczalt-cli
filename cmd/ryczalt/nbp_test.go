package main

import (
	"math/big"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func fakeNBP(t *testing.T, calls *[]string) {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		*calls = append(*calls, r.URL.Path)
		w.Write([]byte(`{"table":"A","currency":"euro","code":"EUR","rates":[
			{"no":"210/A/NBP/2026","effectiveDate":"2026-10-28","mid":4.2501},
			{"no":"211/A/NBP/2026","effectiveDate":"2026-10-29","mid":4.2512},
			{"no":"212/A/NBP/2026","effectiveDate":"2026-10-30","mid":4.3955}]}`))
	}))
	old := nbpURL
	nbpURL = srv.URL
	t.Cleanup(func() { nbpURL = old; srv.Close() })
}

func TestRevenuePLN(t *testing.T) {
	var calls []string
	fakeNBP(t, &calls)
	cache := t.TempDir()
	inv := invoice{
		number:     "INV-11",
		issued:     time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC),
		serviceEnd: time.Date(2026, 10, 31, 0, 0, 0, 0, time.UTC),
		currency:   "EUR",
		total:      big.NewRat(3333, 1),
	}
	for range 2 {
		g, r, err := revenuePLN(inv, "period_end", cache)
		if err != nil {
			t.Fatal(err)
		}
		// 3333 x 4.3955 = 14 650.2015
		if g != 1465020 || r.Table != "212/A/NBP/2026" || r.Date != "2026-10-30" {
			t.Errorf("got %d grosze, %+v", g, r)
		}
	}
	// the day before the revenue date ends the range; the second call is cached
	if len(calls) != 1 || calls[0] != "/eur/2026-10-20/2026-10-30/" {
		t.Errorf("NBP calls: %q", calls)
	}

	pln := inv
	pln.currency = "PLN"
	if g, _, _ := revenuePLN(pln, "period_end", cache); g != 333300 || len(calls) != 1 {
		t.Errorf("PLN invoice: %d grosze, %d calls", g, len(calls))
	}

	future := inv
	y, m, d := time.Now().In(warsaw).Date()
	future.serviceEnd = time.Date(y, m, d+1, 0, 0, 0, 0, time.UTC) // the day before is today
	if _, _, err := revenuePLN(future, "period_end", cache); err == nil {
		t.Error("want an error for a rate not published yet")
	}
}

func TestRevenueDate(t *testing.T) {
	end := time.Date(2026, 10, 31, 0, 0, 0, 0, time.UTC)
	early := invoice{issued: time.Date(2026, 10, 28, 0, 0, 0, 0, time.UTC), serviceEnd: end}
	late := invoice{issued: time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC), serviceEnd: end}
	cases := []struct {
		basis string
		inv   invoice
		want  string
	}{
		{"period_end", early, "2026-10-31"},
		{"period_end", late, "2026-10-31"},
		{"service", early, "2026-10-28"}, // invoiced before the work ended
		{"service", late, "2026-10-31"},
	}
	for _, c := range cases {
		if got := revenueDate(c.basis, c.inv).Format(time.DateOnly); got != c.want {
			t.Errorf("%s, issued %s: got %s, want %s", c.basis, c.inv.issued.Format(time.DateOnly), got, c.want)
		}
	}
}

func TestPitRateRejectsBadData(t *testing.T) {
	var calls []string
	fakeNBP(t, &calls)
	cache := t.TempDir()
	revenue := time.Date(2026, 10, 31, 0, 0, 0, 0, time.UTC)
	path := filepath.Join(cache, "nbp", "eur-2026-10-30.json")
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	// damaged, zero and too-late cache entries are fetched again, not used
	for _, bad := range []string{`{"Mid":"abc","Table":"x","Date":"2026-10-30"}`, `{"Mid":"0","Table":"x","Date":"2026-10-30"}`,
		`{"Mid":"4.1","Table":"x","Date":"2026-10-31"}`, `{"Mid":"4,39"`} {
		if err := os.WriteFile(path, []byte(bad), 0o600); err != nil {
			t.Fatal(err)
		}
		r, err := pitRate("EUR", revenue, cache)
		if err != nil || r.Mid != "4.3955" {
			t.Errorf("cache %s: got %+v, %v", bad, r, err)
		}
	}
	if len(calls) != 4 {
		t.Errorf("want a fetch per damaged entry, got %d", len(calls))
	}
	if _, err := pitRate("../x", revenue, cache); err == nil {
		t.Error("want an error for a currency that is not a code")
	}
}
