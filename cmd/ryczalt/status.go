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

	zusPaid     int64 // social, health and FP/FS recorded as paid, grosze
	zusPaidOn   time.Time
	ryczPaid    bool // any ryczałt payment counts: paying less after deductions is lawful
	ryczPaidOn  time.Time
	ryczPaidAmt int64
}

func (m month) zusDue() amount {
	return amount{m.social.grosze + m.fpfs.grosze + m.health.grosze, min(m.social.status, m.fpfs.status, m.health.status)}
}

// zusOwed is what is left of the month's ZUS, in grosze.
func (m month) zusOwed() int64 { return max(0, m.zusDue().grosze-m.zusPaid) }

func (m month) ryczOwed() bool { return m.invoiced && !m.ryczPaid }

// zusOpen says some of the month's ZUS is still to pay; with the amount
// unknown, only a recorded payment settles it.
func (m month) zusOpen() bool {
	d := m.zusDue()
	return d.status == unknown && m.zusPaid == 0 || m.zusPaid < d.grosze
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
func buildYear(c config, invs []invoice, pays []payment, year int, today time.Time, cacheDir string) ([]month, []string) {
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
		for _, k := range []string{"social", "health", "fpfs"} {
			g, on, _ := paidFor(pays, m, k)
			row.zusPaid += g
			if on.After(row.zusPaidOn) {
				row.zusPaidOn = on
			}
		}
		row.ryczPaidAmt, row.ryczPaidOn, row.ryczPaid = paidFor(pays, m, "ryczalt")
		months = append(months, row)
	}
	return months, notes
}

// monthState says where a month stands on day today, and how loud to say it.
func monthState(m month, today time.Time) (text string, level string) {
	end := m.start.AddDate(0, 1, 0)
	invoiceBy := shiftDeadline("PL", end.AddDate(0, 0, 14)) // the 15th of the next month (art. 106i ust. 1 VAT)
	var unpaid []string
	if m.zusOpen() {
		unpaid = append(unpaid, "zus")
	}
	if m.ryczOwed() {
		unpaid = append(unpaid, "ryczalt")
	}
	late := !today.Before(m.due) && len(unpaid) > 0 && today.After(m.due)
	switch {
	case today.Before(end):
		if m.zusPaid > 0 && !m.zusOpen() {
			return "in progress, zus paid", ""
		}
		return "in progress", ""
	case !m.invoiced && today.After(invoiceBy):
		if late {
			return "no invoice; zus unpaid", "FAIL"
		}
		return "no invoice; due by " + invoiceBy.Format(time.DateOnly), "FAIL"
	case late:
		return strings.Join(unpaid, ", "), "FAIL"
	case len(unpaid) == 0 && m.invoiced:
		if last := maxTime(m.zusPaidOn, m.ryczPaidOn); last.After(m.due) {
			return "late, " + last.Format(time.DateOnly), "WARN"
		}
		return "", "OK"
	case !m.invoiced:
		return "not invoiced yet; due by " + invoiceBy.Format(time.DateOnly), "INFO"
	case today.Equal(m.due):
		return "due today", "WARN"
	}
	days := int(m.due.Sub(today).Hours() / 24)
	text = fmt.Sprintf("due in %d days", days)
	if m.zusPaid > 0 && !m.zusOpen() {
		text += ", zus paid"
	}
	if days <= 7 {
		return text, "WARN"
	}
	return text, ""
}

func maxTime(a, b time.Time) time.Time {
	if a.After(b) {
		return a
	}
	return b
}

func cmdStatus(args []string) error {
	fs := flag.NewFlagSet("status", flag.ContinueOnError)
	cfgFlag := fs.String("c", "", "config file")
	todayFlag := fs.String("today", "", "pretend today is `DATE` (YYYY-MM-DD)")
	yearFlag := fs.Int("year", 0, "show `YEAR` instead of this one")
	if err := parseFlags(fs, args); err != nil {
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
	invs, found, err := readArchive(c.archive, c.nip)
	if err != nil {
		return err
	}
	pays, pwarns, err := readPayments(c.archive)
	if err != nil {
		return err
	}
	months, notes := buildYear(c, invs, pays, year, today, cacheDir())
	found = append(found, asFindings("payments", pwarns)...)
	found = append(found, asFindings("rates", notes)...)
	found = append(found, checkInvoices(invs)...)
	if failed := printStatus(os.Stdout, c, year, today, months, found); failed {
		return errFailed
	}
	return nil
}

// printStatus writes the report and says whether anything failed.
func printStatus(w *os.File, c config, year int, today time.Time, months []month, found []finding) bool {
	phase := zusPhase(c, today) + " ZUS"
	if c.chorobowa {
		phase += " with chorobowa"
	}
	fmt.Fprintln(w, paint(w, bold, fmt.Sprintf("ryczalt %d", year)))
	fmt.Fprintln(w, paint(w, dim, fmt.Sprintf("%s %s | %s%% ryczalt | %s | rules %s\n",
		today.Weekday().String()[:3], today.Format(time.DateOnly), c.rate, phase, rulesStatus(year))))
	if len(months) == 0 {
		fmt.Fprintf(w, "No months of business in %d up to today.\n", year)
		return false
	}

	// the next month to pay for: the first with anything left to pay
	next := -1
	for i, m := range months {
		if m.zusOpen() || m.ryczOwed() {
			next = i
			break
		}
	}

	const colState = 7
	rows := [][]string{{"Month", "Invoice", "Revenue PLN", "Ryczalt*", "Social", "Health", "Due", "State"}}
	levels := []string{""}
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
			text = strings.TrimSpace(tag(w, map[string]string{"FAIL": red, "WARN": yellow, "INFO": cyan, "OK": green}[level], level) + " " + text)
		}
		failed = failed || level == "FAIL"
		rows = append(rows, []string{m.start.Format("2006-01"), inv, revenue, ryczalt, social,
			plnText(m.health), m.due.Weekday().String()[:3] + " " + m.due.Format(time.DateOnly), text})
		levels = append(levels, level)
	}
	right := []bool{false, false, true, true, true, true, false, false}
	printTable(w, rows, right, func(r, col int) string {
		cell := rows[r][col]
		switch {
		case r == 0:
			return bold
		case col == colState && levels[r] != "":
			return "" // the tag carries its own colour
		case levels[r] == "OK": // settled: history
			return dim
		case cell == "n/a" || cell == "-":
			return dim
		case strings.HasPrefix(cell, "~"):
			return yellow
		case r-1 == next && (col == 3 || col == 4 || col == 5 || col == 6):
			return bold
		case col == colState:
			return ""
		}
		return ""
	})
	fmt.Fprintln(w, "\n"+paint(w, dim, "* in full, before deducting paid ZUS (social, half of health); PIT-28 settles the deductions"))

	// year so far
	revenue := amount{0, official}
	for _, m := range months {
		revenue.grosze += m.revenue.grosze
		revenue.status = min(revenue.status, m.revenue.status)
	}
	label := func(s string) string { return paint(w, dim, fmt.Sprintf("%-13s", s)) }
	fmt.Fprintf(w, "\n%s %s PLN revenue, health bracket %s\n", label("Year so far"),
		paint(w, bold, plnText(revenue)), bracketName(months[len(months)-1].bracket))

	if next >= 0 {
		m := months[next]
		owed := amount{m.zusOwed(), m.zusDue().status}
		zus := "paid " + m.zusPaidOn.Format(time.DateOnly)
		if m.zusOpen() {
			zus = plnText(owed) + " PLN"
			if m.zusPaid == 0 {
				zus += fmt.Sprintf(" (social %s, health %s", plnText(m.social), plnText(m.health))
				if m.fpfs.grosze != 0 {
					zus += ", FP/FS " + plnText(m.fpfs)
				}
				zus += ")"
			} else {
				zus += " left of " + plnText(m.zusDue())
			}
		}
		total := owed
		tax := "not yet invoiced"
		switch {
		case m.ryczPaid:
			tax = "paid " + m.ryczPaidOn.Format(time.DateOnly)
		case m.ryczalt.status != unknown:
			tax = strings.TrimSuffix(plnText(m.ryczalt), ".00") + " PLN*"
			total = amount{total.grosze + m.ryczalt.grosze, min(total.status, m.ryczalt.status)}
		}
		head := label("Next payment")
		if m.due.Before(today) {
			head = paint(w, red, fmt.Sprintf("%-13s", "Overdue"))
		}
		fmt.Fprintf(w, "\n%s %s, for %s\n", head,
			paint(w, bold, m.due.Weekday().String()[:3]+" "+m.due.Format(time.DateOnly)), m.start.Format("2006-01"))
		fmt.Fprintf(w, "%s %s\n", label("  zus"), zus)
		fmt.Fprintf(w, "%s %s\n", label("  ryczalt"), tax)
		fmt.Fprintf(w, "%s %s\n", label("  total"), paint(w, bold, plnText(total)+" PLN"))
	}

	anyPaid, lastDue := false, -1
	for i, m := range months {
		anyPaid = anyPaid || m.zusPaid > 0 || m.ryczPaid
		if m.due.Before(today) {
			lastDue = i
		}
	}
	if !anyPaid && lastDue >= 0 {
		rng := months[0].start.Format("2006-01")
		if lastDue > 0 {
			rng += ".." + months[lastDue].start.Format("2006-01")
		}
		fmt.Fprintf(w, "\n%s no payments recorded yet; if all were paid on time: ryczalt paid --on-due %s\n",
			tag(w, cyan, "INFO"), rng)
	}

	more := 0
	for _, f := range found {
		if f.level != levelWarn {
			more++
		}
	}
	if more < len(found) {
		fmt.Fprintln(w)
		printFindings(w, found, levelWarn)
	}
	if more > 0 {
		fmt.Fprintf(w, "\n%s", paint(w, dim, fmt.Sprintf("%d more notes about the archive: ryczalt check\n", more)))
	}
	return failed
}

func asFindings(group string, texts []string) []finding {
	var out []finding
	for _, t := range texts {
		out = append(out, finding{levelWarn, group, t})
	}
	return out
}

func cmdCheck(args []string) error {
	flags := flag.NewFlagSet("check", flag.ContinueOnError)
	cfgFlag := flags.String("c", "", "config file")
	if err := parseFlags(flags, args); err != nil {
		return exitError{2, err}
	}
	if flags.NArg() > 0 {
		return exitError{2, errors.New("usage: ryczalt check [-c CONFIG]")}
	}
	c, err := loadConfig(*cfgFlag)
	if err != nil {
		return err
	}
	invs, found, err := readArchive(c.archive, c.nip)
	if err != nil {
		return err
	}
	pays, pwarns, err := readPayments(c.archive)
	if err != nil {
		return err
	}
	found = append(found, asFindings("payments", pwarns)...)
	found = append(found, checkInvoices(invs)...)
	w := os.Stdout
	fmt.Fprintln(w, paint(w, bold, "ryczalt check"))
	fmt.Fprintln(w, paint(w, dim, fmt.Sprintf("%s | %d invoices | %d payments\n", c.archive, len(invs), len(pays))))
	if len(found) == 0 {
		fmt.Fprintln(w, tag(w, green, "OK"), "nothing to report")
		return nil
	}
	if printFindings(w, found, levelWarn, levelCheck, levelInfo) {
		return errFailed
	}
	return nil
}

func bracketName(b int) string { return [...]string{"up to 60k", "60k-300k", "above 300k"}[b] }

// rulesStatus is the weakest status among the year's figures.
func rulesStatus(year int) status {
	st := official
	for _, k := range ruleKeys {
		_, s := lookup(year, k[0])
		st = min(st, s)
	}
	return st
}
