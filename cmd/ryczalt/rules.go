package main

import (
	"errors"
	"fmt"
	"math/big"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// status says how far a value can be trusted.
// A derived amount takes the weakest status among its inputs.
type status int

const (
	unknown  status = iota // not published yet; dependent amounts are n/a
	forecast               // from a draft or projection; printed with ~
	official               // published in law or by GUS/ZUS
)

func (s status) String() string { return [...]string{"unknown", "forecast", "official"}[s] }

type rule struct {
	value  string // decimal PLN
	status status
	source string
}

const zusRates2026 = "https://www.zus.pl/baza-wiedzy/skladki-wskazniki-odsetki/skladki/wysokosc-skladek-na-ubezpieczenia-spoleczne"

// rules are the yearly figures the contributions are computed from.
// Each January the new year's health_wage is added once GUS publishes it, and
// forecasts are replaced by the official figures; that ships as a patch release.
// A missing year or key is unknown: last year's value is not carried over.
var rules = map[int]map[string]rule{
	2026: {
		"minimum_wage":  {"4806", official, zusRates2026},
		"forecast_wage": {"9420", official, zusRates2026},
		"health_wage":   {"9228.64", official, "https://www.biznes.gov.pl/pl/portal/001052"},
	},
	2027: {
		"minimum_wage":  {"4950", official, "Dz.U. 2026 poz. 1213"},
		"forecast_wage": {"10033", forecast, "draft 2027 budget"},
	},
}

// ruleKeys in print order, with what each one is.
var ruleKeys = [][2]string{
	// minimalne wynagrodzenie; 30% of it is the preferential ZUS base
	{"minimum_wage", "minimum wage"},
	// prognozowane przeciętne wynagrodzenie; 60% of it is the full ZUS base
	{"forecast_wage", "forecast average wage"},
	// przeciętne wynagrodzenie w sektorze przedsiębiorstw w IV kwartale
	// poprzedniego roku, z wypłatami z zysku; the ryczałt health base
	{"health_wage", "average wage, Q4 of the year before"},
}

// staleRules lists the figures of now's year that should be official by
// now: GUS and ZUS publish them by the end of January.
func staleRules(now time.Time) []string {
	if now.Month() == time.January {
		return nil
	}
	var out []string
	for _, k := range ruleKeys {
		if _, st := lookup(now.Year(), k[0]); st != official {
			out = append(out, fmt.Sprintf("%d %s is %s", now.Year(), k[0], st))
		}
	}
	return out
}

// Social contribution rates (ustawa o systemie ubezpieczeń społecznych,
// art. 22). They have not changed for years; move them into rules if
// one does.
var (
	rateEmerytalna = big.NewRat(1952, 10000)
	rateRentowa    = big.NewRat(800, 10000)
	rateWypadkowa  = big.NewRat(167, 10000) // payers with up to 9 insured
	rateChorobowa  = big.NewRat(245, 10000) // voluntary for the self-employed
	rateFPFS       = big.NewRat(245, 10000) // Fundusz Pracy 1.00% + Fundusz Solidarnościowy 1.45%
	rateHealth     = big.NewRat(9, 100)
)

// amount is a sum in grosze with the status of the rules behind it.
type amount struct {
	grosze int64
	status status
}

func lookup(year int, key string) (*big.Rat, status) {
	r, ok := rules[year][key]
	if !ok {
		return nil, unknown
	}
	v, ok := new(big.Rat).SetString(r.value)
	if !ok {
		panic(fmt.Sprintf("rules %d %s: bad value %q", year, key, r.value))
	}
	return v, r.status
}

// grosze rounds a PLN amount to grosze, halves away from zero, so a
// correction rounds the same way as the invoice it corrects.
func grosze(pln *big.Rat) int64 {
	g := new(big.Rat).Mul(new(big.Rat).Abs(pln), big.NewRat(100, 1))
	g.Add(g, big.NewRat(1, 2))
	n := new(big.Int).Quo(g.Num(), g.Denom()).Int64()
	if pln.Sign() < 0 {
		return -n
	}
	return n
}

func ratOf(g int64) *big.Rat { return big.NewRat(g, 100) }

// contribution rounds the base to grosze, then each rate's contribution,
// the way ZUS computes them.
func contribution(base *big.Rat, rates ...*big.Rat) int64 {
	b := ratOf(grosze(base))
	var sum int64
	for _, r := range rates {
		sum += grosze(new(big.Rat).Mul(b, r))
	}
	return sum
}

// healthShares are the ryczałt health bases by yearly revenue, as shares of
// health_wage: up to 60 000 PLN, up to 300 000 PLN, above (ustawa o
// świadczeniach opieki zdrowotnej, art. 81 ust. 2h).
var healthShares = []*big.Rat{big.NewRat(60, 100), big.NewRat(100, 100), big.NewRat(180, 100)}

// healthMonthly is the ryczałt health contribution for bracket 0, 1 or 2.
func healthMonthly(year, bracket int) amount {
	w, st := lookup(year, "health_wage")
	if st == unknown {
		return amount{}
	}
	return amount{contribution(new(big.Rat).Mul(w, healthShares[bracket]), rateHealth), st}
}

// socialMonthly is the social contribution in the preferential or the full phase
// (none is due during ulga na start). fpfs is Fundusz Pracy and Fundusz Solidarnościowy
// , due only in the full phase and kept apart because it cannot be deducted from revenue.
func socialMonthly(year int, preferential, chorobowa bool) (social, fpfs amount) {
	key, share := "forecast_wage", big.NewRat(60, 100)
	if preferential {
		key, share = "minimum_wage", big.NewRat(30, 100)
	}
	w, st := lookup(year, key)
	if st == unknown {
		return amount{}, amount{}
	}
	base := new(big.Rat).Mul(w, share)
	rates := []*big.Rat{rateEmerytalna, rateRentowa, rateWypadkowa}
	if chorobowa {
		rates = append(rates, rateChorobowa)
	}
	social, fpfs = amount{contribution(base, rates...), st}, amount{0, st}
	if !preferential {
		fpfs.grosze = contribution(base, rateFPFS)
	}
	return social, fpfs
}

// plnText writes grosze as "1 495.04", with ~ in front of a forecast and
// n/a for an unknown amount.
func plnText(a amount) string {
	if a.status == unknown {
		return "n/a"
	}
	g, sign := a.grosze, ""
	if g < 0 {
		g, sign = -g, "-"
	}
	s := fmt.Sprintf("%d.%02d", g/100, g%100)
	for i := strings.IndexByte(s, '.') - 3; i > 0; i -= 3 {
		s = s[:i] + " " + s[i:]
	}
	if a.status == forecast {
		sign = "~" + sign
	}
	return sign + s
}

var yearArg = regexp.MustCompile(`^20\d\d$`)

func cmdRules(args []string) error {
	year := time.Now().In(warsaw).Year()
	switch {
	case len(args) == 1 && (args[0] == "-h" || args[0] == "--help"):
		fmt.Print(usage)
		return nil
	case len(args) == 1:
		if !yearArg.MatchString(args[0]) {
			return fmt.Errorf("rules takes a year like 2026, got %q", args[0])
		}
		year, _ = strconv.Atoi(args[0])
	case len(args) > 1:
		return errors.New("usage: ryczalt rules [YEAR]")
	}
	printRules(os.Stdout, year)
	return nil
}

func printRules(w *os.File, year int) {
	fmt.Fprintf(w, "Rules for %d, in PLN\n", year)
	var unknowns []string
	hasForecast := false
	for _, k := range ruleKeys {
		v, st := lookup(year, k[0])
		val := "n/a"
		if st != unknown {
			val = plnText(amount{grosze(v), st})
		}
		hasForecast = hasForecast || st == forecast
		if st == unknown {
			unknowns = append(unknowns, k[1])
		}
		fmt.Fprintln(
			w,
			strings.TrimRight(
				fmt.Sprintf("  %-36s %10s  %-8s  %s", k[1], val, st, rules[year][k[0]].source),
				" ",
			),
		)
	}

	fmt.Fprintln(w, "\nMonthly contributions")
	row := func(label string, a amount, note string) {
		fmt.Fprintln(
			w,
			strings.TrimRight(fmt.Sprintf("  %-36s %10s  %s", label, plnText(a), note), " "),
		)
	}
	row("health, yearly revenue up to 60 000", healthMonthly(year, 0), "")
	row("health, up to 300 000", healthMonthly(year, 1), "")
	row("health, above 300 000", healthMonthly(year, 2), "")
	pref, _ := socialMonthly(year, true, true)
	prefNo, _ := socialMonthly(year, true, false)
	without := func(a amount) string {
		if a.status == unknown {
			return ""
		}
		return plnText(a) + " without chorobowa"
	}
	row("social, preferential", pref, without(prefNo))
	full, fpfs := socialMonthly(year, false, true)
	fullNo, _ := socialMonthly(year, false, false)
	row("social, full", full, without(fullNo))
	row("FP/FS, full phase only", fpfs, "")

	for _, u := range unknowns {
		fmt.Fprintf(
			w,
			"%s no %d figure for %s; what depends on it shows n/a\n",
			tag(w, yellow, "WARN"),
			year,
			u,
		)
	}
	if hasForecast {
		fmt.Fprintf(
			w,
			"%s ~ marks an amount based on a forecast, not the official figure\n",
			tag(w, cyan, "INFO"),
		)
	}
}
