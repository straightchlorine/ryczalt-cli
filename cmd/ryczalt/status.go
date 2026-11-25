package main

import (
	"errors"
	"flag"
	"fmt"
	"math/big"
	"os"
	"strings"
	"time"
)

// errFailed makes main exit 1 without printing anything more: the report
// already says what failed.
var errFailed = errors.New("something needs attention")

// month is one calendar month of business: ZUS is due for it whether or
// not it was invoiced, ryczałt only on what was invoiced.
type month struct {
	start    time.Time // first day, UTC midnight
	invoices []string
	revenue  amount // grosze; unknown when a rate could not be had
	invoiced bool
	ryczalt  amount // whole PLN in grosze; before deductions of paid ZUS
	social   amount // emerytalna, rentowa, wypadkowa, chorobowa
	fpfs     amount
	health   amount
	bracket  int // 0, 1, 2: up to 60 000, up to 300 000, above
	due      time.Time
}

// zusPhase is the social contribution phase in month m.
func zusPhase(c config, m time.Time) string {
	switch {
	case !c.ulgaUntil.IsZero() && !m.After(c.ulgaUntil):
		return "ulga"
	case !c.preferentialUntil.IsZero() && !m.After(c.preferentialUntil):
		return "preferential"
	}
	return "full"
}

// healthThresholds are the yearly revenue limits of the health brackets
// (ustawa o świadczeniach opieki zdrowotnej, art. 81 ust. 2h), in grosze.
var healthThresholds = [2]int64{60_000_00, 300_000_00}

// wholePLN rounds grosze to whole złoty, half up (Ordynacja podatkowa
// art. 63 § 1), for tax bases and tax.
func wholePLN(g int64) int64 {
	if g < 0 {
		return -wholePLN(-g)
	}
	return (g + 50) / 100 * 100
}

// buildYear works out every month of year up to today's month.
func buildYear(c config, invs []invoice, year int, today time.Time, cacheDir string) ([]month, []string) {
	var notes []string
	first := time.Date(year, 1, 1, 0, 0, 0, 0, time.UTC)
	if c.businessStart.After(first) {
		first = c.businessStart
	}
	thisMonth := time.Date(today.Year(), today.Month(), 1, 0, 0, 0, 0, time.UTC)

	byMonth := map[time.Time][]invoice{}
	for _, inv := range invs {
		d := revenueDate(c.revenueBasis, inv)
		m := time.Date(d.Year(), d.Month(), 1, 0, 0, 0, 0, time.UTC)
		byMonth[m] = append(byMonth[m], inv)
	}

	rate, _ := new(big.Rat).SetString(c.rate)
	rate.Quo(rate, big.NewRat(100, 1))
	var months []month
	cumRevenue, cumSocial, prevTax := amount{0, official}, int64(0), int64(0)
	for m := first; m.Year() == year && !m.After(thisMonth); m = m.AddDate(0, 1, 0) {
		row := month{start: m, revenue: amount{0, official}}
		for _, inv := range byMonth[m] {
			row.invoices = append(row.invoices, inv.number)
			row.invoiced = true
			g, _, err := revenuePLN(inv, c.revenueBasis, cacheDir)
			if err != nil {
				notes = append(notes, fmt.Sprintf("%s: %v", inv.number, err))
				row.revenue.status = unknown
				continue
			}
			row.revenue.grosze += g
		}

		switch zusPhase(c, m) {
		case "ulga":
			row.social, row.fpfs = amount{0, official}, amount{0, official}
		case "preferential":
			row.social, row.fpfs = socialMonthly(year, true, c.chorobowa)
		default:
			row.social, row.fpfs = socialMonthly(year, false, c.chorobowa)
		}

		// the bracket follows revenue from 1 January less the social
		// contributions; ponytail: those are assumed paid on time until
		// payments are recorded
		cumRevenue.grosze += row.revenue.grosze
		cumRevenue.status = min(cumRevenue.status, row.revenue.status)
		cumSocial += row.social.grosze
		base := cumRevenue.grosze - cumSocial
		for row.bracket < 2 && base > healthThresholds[row.bracket] {
			row.bracket++
		}
		row.health = healthMonthly(year, row.bracket)
		if cumRevenue.status == unknown {
			row.health.status = unknown
		}

		// ryczałt is worked out from 1 January (art. 21 ustawy o
		// zryczałtowanym podatku): rate x base, less what earlier months owe
		tax := grosze(new(big.Rat).Mul(ratOf(wholePLN(cumRevenue.grosze)), rate))
		tax = wholePLN(tax)
		row.ryczalt = amount{tax - prevTax, cumRevenue.status}
		if !row.invoiced {
			row.ryczalt.status = unknown
		}
		prevTax = tax

		row.due = shiftDeadline("PL", time.Date(year, m.Month()+1, 20, 0, 0, 0, 0, time.UTC))
		months = append(months, row)
	}
	return months, notes
}

// monthState says where a month stands on day today. fail is set when
// something is overdue that ryczalt can see without a payment log.
func monthState(m month, today time.Time) (text string, level string) {
	end := m.start.AddDate(0, 1, 0)
	invoiceBy := shiftDeadline("PL", end.AddDate(0, 0, 14)) // the 15th of the next month (art. 106i ust. 1 VAT)
	switch {
	case today.Before(end):
		return "in progress", ""
	case !m.invoiced && today.After(invoiceBy):
		return "no invoice; due by " + invoiceBy.Format(time.DateOnly), "FAIL"
	case !m.invoiced:
		return "not invoiced yet; due by " + invoiceBy.Format(time.DateOnly), "INFO"
	case today.After(m.due):
		return "past due date", ""
	case today.Equal(m.due):
		return "due today", "WARN"
	}
	days := int(m.due.Sub(today).Hours() / 24)
	if days <= 7 {
		return fmt.Sprintf("due in %d days", days), "WARN"
	}
	return fmt.Sprintf("due in %d days", days), ""
}

func cmdStatus(args []string) error {
	fs := flag.NewFlagSet("status", flag.ContinueOnError)
	cfgFlag := fs.String("c", "", "config file")
	todayFlag := fs.String("today", "", "pretend today is `DATE` (YYYY-MM-DD)")
	yearFlag := fs.Int("year", 0, "show `YEAR` instead of this one")
	if err := fs.Parse(args); err != nil {
		return exitError{2, err}
	}
	if fs.NArg() > 0 {
		return exitError{2, fmt.Errorf("status takes no arguments, got %q", fs.Args())}
	}
	y, mo, d := time.Now().In(warsaw).Date()
	today := time.Date(y, mo, d, 0, 0, 0, 0, time.UTC)
	if *todayFlag != "" {
		t, err := time.Parse(time.DateOnly, *todayFlag)
		if err != nil {
			return exitError{2, fmt.Errorf("--today wants a date like 2026-11-20, got %q", *todayFlag)}
		}
		today = t
	}
	year := today.Year()
	if *yearFlag != 0 {
		year = *yearFlag
	}

	c, err := loadConfig(*cfgFlag)
	if err != nil {
		return err
	}
	invs, warns, err := readArchive(c.archive, c.nip)
	if err != nil {
		return err
	}
	months, notes := buildYear(c, invs, year, today, xdgPath("XDG_CACHE_HOME", ".cache", "ryczalt"))
	if failed := printStatus(os.Stdout, c, year, today, months, append(warns, notes...)); failed {
		return errFailed
	}
	return nil
}

// printStatus writes the report and says whether anything failed.
func printStatus(w *os.File, c config, year int, today time.Time, months []month, warns []string) bool {
	fmt.Fprintf(w, "ryczalt %s  today %s  ryczalt %s%%  zus %s  rules %d %s\n\n",
		version, today.Format(time.DateOnly), c.rate, zusPhase(c, today), year, rulesStatus(year))
	if len(months) == 0 {
		fmt.Fprintf(w, "No months of business in %d up to today.\n", year)
		return false
	}

	line := func(cols ...string) {
		fmt.Fprintln(w, strings.TrimRight(fmt.Sprintf("%-8s %-16s %12s %9s %8s %8s  %-10s  %s", toAny(cols)...), " "))
	}
	line("Month", "Invoice", "Revenue PLN", "Ryczalt*", "Social", "Health", "Due", "State")
	failed := false
	for _, m := range months {
		inv := "-"
		if len(m.invoices) > 0 {
			inv = m.invoices[0]
			if len(m.invoices) > 1 {
				inv += fmt.Sprintf(" +%d", len(m.invoices)-1)
			}
		}
		revenue, ryczalt := plnText(m.revenue), plnText(m.ryczalt)
		if !m.invoiced {
			revenue = "-"
		}
		if m.ryczalt.status != unknown {
			ryczalt = strings.TrimSuffix(ryczalt, ".00")
		}
		social := plnText(amount{m.social.grosze + m.fpfs.grosze, min(m.social.status, m.fpfs.status)})
		text, level := monthState(m, today)
		if level != "" {
			text = tag(w, map[string]string{"FAIL": red, "WARN": yellow, "INFO": cyan}[level], level) + " " + text
		}
		failed = failed || level == "FAIL"
		line(m.start.Format("2006-01"), inv, revenue, ryczalt, social, plnText(m.health), m.due.Format(time.DateOnly), text)
	}
	fmt.Fprintln(w, "\n* ryczalt before deducting paid ZUS (social, and half of health); "+
		"the payment log will lower it")

	// year summary
	revenue := amount{0, official}
	for _, m := range months {
		revenue.grosze += m.revenue.grosze
		revenue.status = min(revenue.status, m.revenue.status)
	}
	fmt.Fprintf(w, "\nYear %d so far: revenue %s PLN; health bracket %s", year, plnText(revenue), bracketName(months[0].bracket))
	for i := 1; i < len(months); i++ {
		if months[i].bracket != months[i-1].bracket {
			fmt.Fprintf(w, ", %s from %s", bracketName(months[i].bracket), months[i].start.Format("2006-01"))
		}
	}
	fmt.Fprintln(w)

	// the next payment that is not yet past
	for _, m := range months {
		if m.due.Before(today) {
			continue
		}
		zus := amount{m.social.grosze + m.fpfs.grosze + m.health.grosze, min(m.social.status, m.fpfs.status, m.health.status)}
		fmt.Fprintf(w, "Next payments by %s %s: ZUS %s PLN for %s", m.due.Weekday().String()[:3], m.due.Format(time.DateOnly),
			plnText(zus), m.start.Format("2006-01"))
		if m.ryczalt.status != unknown {
			fmt.Fprintf(w, ", ryczalt up to %s PLN", strings.TrimSuffix(plnText(m.ryczalt), ".00"))
		} else {
			fmt.Fprint(w, ", ryczalt once the month is invoiced")
		}
		fmt.Fprintln(w)
		break
	}

	if c.methodConfirmedOn == "" {
		warns = append(warns, "revenue_basis is not confirmed by an accountant yet (tax.method_confirmed_on)")
	}
	if len(warns) > 0 {
		fmt.Fprintln(w)
	}
	for _, s := range warns {
		fmt.Fprintln(w, tag(w, yellow, "WARN"), s)
	}
	return failed
}

func bracketName(b int) string {
	return [...]string{"up to 60 000", "60 000-300 000", "above 300 000"}[b]
}

// rulesStatus is the weakest status among the year's figures.
func rulesStatus(year int) status {
	st := official
	for _, k := range ruleKeys {
		_, s := lookup(year, k[0])
		st = min(st, s)
	}
	return st
}

func toAny(s []string) []any {
	out := make([]any, len(s))
	for i, v := range s {
		out[i] = v
	}
	return out
}
