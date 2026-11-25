package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"math/big"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

var nbpURL = "https://api.nbp.pl/api/exchangerates/rates/a"

// nbpRate is a mid rate from an NBP table A.
type nbpRate struct {
	Mid   string // PLN per unit, e.g. "4.2322"
	Table string // e.g. "212/A/NBP/2026"
	Date  string // day the table was published
}

// revenueDate is the day revenue arises
// (art. 14 ust. 1c and 1e ustawy o PIT,
// applied to ryczałt by art. 6 ust. 1 ustawy o zryczałtowanym podatku).

// NOTE: since under "service" an earlier payment would also count, payments
// from the client are not recorded yet.
func revenueDate(basis string, inv invoice) time.Time {
	if basis == "service" && inv.issued.Before(inv.serviceEnd) {
		return inv.issued
	}
	return inv.serviceEnd
}

// pitRate is the rate for converting revenue in a foreign currency: the
// NBP table A published on the last business day before the revenue date
// (art. 11a ust. 1 ustawy o PIT). Answer cached in cacheDir.
func pitRate(currency string, revenue time.Time, cacheDir string) (nbpRate, error) {
	// it goes into a URL and a file name
	if !currencyCode.MatchString(currency) {
		return nbpRate{}, fmt.Errorf("currency %q is not a three-letter code", currency)
	}
	before := revenue.AddDate(0, 0, -1)
	// today's table may not be out yet, and an answer that can still change
	// must not be cached
	y, m, d := time.Now().In(warsaw).Date()
	if !before.Before(time.Date(y, m, d, 0, 0, 0, 0, time.UTC)) {
		return nbpRate{}, fmt.Errorf(
			"the %s rate for revenue on %s is not published yet",
			currency,
			revenue.Format(time.DateOnly),
		)
	}
	cache := filepath.Join(
		cacheDir,
		"nbp",
		strings.ToLower(currency)+"-"+before.Format(time.DateOnly)+".json",
	)
	var r nbpRate
	b, err := os.ReadFile(cache)
	if err == nil && json.Unmarshal(b, &r) == nil && r.valid(before) {
		return r, nil
	}
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return nbpRate{}, err
	} // a damaged cache entry is fetched again and overwritten

	// a range, so a weekend or holiday resolves to the table before it
	u := fmt.Sprintf("%s/%s/%s/%s/?format=json", nbpURL, strings.ToLower(currency),
		before.AddDate(0, 0, -10).Format(time.DateOnly), before.Format(time.DateOnly))
	req, err := http.NewRequest("GET", u, nil)
	if err != nil {
		return nbpRate{}, err
	}
	req.Header.Set("User-Agent", "ryczalt/"+version)
	resp, err := (&http.Client{Timeout: 15 * time.Second}).Do(req)
	if err != nil {
		return nbpRate{}, fmt.Errorf("NBP rate lookup: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nbpRate{}, fmt.Errorf("NBP rate lookup: %s for %s", resp.Status, u)
	}
	var body struct {
		Rates []struct {
			No, EffectiveDate string
			Mid               json.Number
		}
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return nbpRate{}, fmt.Errorf("NBP rate lookup: %w", err)
	}
	if len(body.Rates) == 0 {
		return nbpRate{}, fmt.Errorf(
			"NBP has no %s rate in the ten days before %s",
			currency,
			revenue.Format(time.DateOnly),
		)
	}
	last := body.Rates[len(body.Rates)-1]
	r = nbpRate{Mid: last.Mid.String(), Table: last.No, Date: last.EffectiveDate}
	if !r.valid(before) {
		return nbpRate{}, fmt.Errorf("NBP sent an unusable rate: %+v", r)
	}

	b, _ = json.Marshal(r)
	if err := os.MkdirAll(filepath.Dir(cache), 0o700); err == nil {
		os.WriteFile(cache, b, 0o600) // a failed write only costs a lookup next time
	}
	return r, nil
}

// valid reports a positive rate from a table published on or before day.
func (r nbpRate) valid(day time.Time) bool {
	mid, ok := new(big.Rat).SetString(r.Mid)
	published, err := time.Parse(time.DateOnly, r.Date)
	return ok && mid.Sign() > 0 && r.Table != "" && err == nil && !published.After(day)
}

// revenuePLN is the invoice's revenue in grosze: its total, converted
// with the PIT rate unless it is already in PLN.
func revenuePLN(inv invoice, basis, cacheDir string) (int64, nbpRate, error) {
	if inv.currency == "PLN" {
		return grosze(inv.total), nbpRate{}, nil
	}
	r, err := pitRate(inv.currency, revenueDate(basis, inv), cacheDir)
	if err != nil {
		return 0, r, err
	}
	mid, _ := new(big.Rat).SetString(r.Mid) // checked by valid
	return grosze(new(big.Rat).Mul(inv.total, mid)), r, nil
}
