package main

import (
	"errors"
	"flag"
	"fmt"
	"math/big"
	"os"
	"slices"
	"strconv"
	"strings"
	"time"
)

// yearSheet is a tax year settled: the PIT-28 figures and the annual
// health contribution settlement, all in grosze.
type yearSheet struct {
	year     int
	months   int    // months of business in the year
	complete bool   // every month over and invoiced
	revenue  amount // revenue whose date falls in the year

	socialPaid int64 // social contributions paid in the year (art. 11 ust. 1 ustawy o ryczałcie)
	healthPaid int64 // health contributions paid in the year; half of it is deducted (ust. 1a)
	deductions int64 // capped at revenue: what is left over is lost
	base       int64 // whole PLN
	tax        int64 // whole PLN
	advances   int64 // ryczałt paid for the year's months, December's included
	result     int64 // tax - advances: > 0 to pay, < 0 refund

	healthBase     int64  // revenue less social paid, picks the yearly bracket
	bracket        int    // 0, 1, 2
	healthYear     amount // months x the yearly bracket's contribution
	healthMonthly  int64  // what was due month by month
	healthSettle   int64  // healthYear - healthMonthly: > 0 top-up, < 0 refund on request
	healthPaidDues int64  // health paid for the year's months, any time
}

// settleYear works the year out from its months and the recorded payments.
func settleYear(c config, months []month, pays []payment, year int, today time.Time) yearSheet {
	s := yearSheet{year: year, revenue: amount{0, official}, complete: true}
	rate := map[string]int64{"17": 1700, "15": 1500, "14": 1400, "12.5": 1250, "12": 1200, "10": 1000,
		"8.5": 850, "5.5": 550, "3": 300}[c.rate] // basis points
	for _, m := range months {
		if m.start.Year() != year {
			continue
		}
		s.months++
		s.revenue.grosze += m.revenue.grosze
		s.revenue.status = min(s.revenue.status, m.revenue.status)
		s.healthMonthly += m.health.grosze
		if !m.invoiced || today.Before(m.start.AddDate(0, 1, 0)) {
			s.complete = false
		}
	}
	if len(months) == 0 || months[len(months)-1].start.Month() != time.December {
		s.complete = false
	}
	for _, p := range pays {
		if p.paidOn.Year() == year {
			switch p.kind {
			case "social":
				s.socialPaid += p.grosze
			case "health":
				s.healthPaid += p.grosze
			}
		}
		if p.period.Year() == year {
			switch p.kind {
			case "ryczalt":
				s.advances += p.grosze
			case "health":
				s.healthPaidDues += p.grosze
			}
		}
	}

	s.deductions = min(s.socialPaid+(s.healthPaid+1)/2, max(s.revenue.grosze, 0))
	s.base = wholePLN(s.revenue.grosze - s.deductions)
	s.tax = wholePLN((s.base*rate + 5000) / 10000)
	s.result = s.tax - s.advances

	s.healthBase = s.revenue.grosze - s.socialPaid
	for s.bracket < 2 && s.healthBase > healthThresholds[s.bracket] {
		s.bracket++
	}
	h := healthMonthly(year, s.bracket)
	s.healthYear = amount{int64(s.months) * h.grosze, h.status}
	s.healthSettle = s.healthYear.grosze - s.healthMonthly
	return s
}

func cmdYear(args []string) error {
	flags := flag.NewFlagSet("year", flag.ContinueOnError)
	cfgFlag := flags.String("c", "", "config file")
	todayFlag := flags.String("today", "", "pretend today is `DATE`")
	if err := parseFlags(flags, args); err != nil {
		return exitError{2, err}
	}
	today := todayWarsaw()
	if *todayFlag != "" {
		t, err := time.Parse(time.DateOnly, *todayFlag)
		if err != nil {
			return exitError{2, fmt.Errorf("--today wants a date like 2027-03-01, got %q", *todayFlag)}
		}
		today = t
	}
	// until the health settlement in May, the year to settle is last year
	year := today.Year()
	if today.Month() <= time.May {
		year--
	}
	switch {
	case flags.NArg() == 1 && yearArg.MatchString(flags.Arg(0)):
		year, _ = strconv.Atoi(flags.Arg(0)) // four digits, checked
	case flags.NArg() > 0:
		return exitError{2, errors.New("usage: ryczalt year [-c CONFIG] [YEAR]")}
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
	if f, ok := healthCliff(c, months, pays, year, today); ok {
		found = append(found, f)
	}
	s := settleYear(c, months, pays, year, today)
	// last December's ZUS is paid in January and deducted this year
	if dec := time.Date(year-1, 12, 1, 0, 0, 0, 0, time.UTC); c.businessStart.Before(dec.AddDate(0, 1, 0)) {
		if _, _, ok := paidFor(pays, dec, "social"); !ok {
			if _, _, ok := paidFor(pays, dec, "health"); !ok {
				found = append(found, finding{levelWarn, "payments", fmt.Sprintf(
					"nothing recorded for %d-12: its ZUS, paid in January %d, is deducted in %d; "+
						"record it, e.g. ryczalt paid %d-12 social AMOUNT --on %d-01-20 (and health)",
					year-1, year, year, year-1, year)})
			}
		}
	}

	w := newReport(os.Stdout)
	printYear(w, c, s, months, pays, today)
	if hasWarn := len(found) > 0; hasWarn {
		w.println()
		printFindings(w, found, levelWarn)
	}
	return w.flush()
}

// monthRanges writes months as "Jan-Oct" or "Jan-Mar, May"; a month of
// another year than year carries it, as in "Dec 2025".
func monthRanges(ms []time.Time, year int) string {
	name := func(m time.Time) string {
		if m.Year() != year {
			return m.Format("Jan 2006")
		}
		return m.Format("Jan")
	}
	var parts []string
	for i := 0; i < len(ms); {
		j := i
		for j+1 < len(ms) && ms[j+1].Equal(ms[j].AddDate(0, 1, 0)) {
			j++
		}
		switch {
		case i == j:
			parts = append(parts, name(ms[i]))
		case ms[i].Year() != ms[j].Year(): // across a new year: both ends carry theirs
			parts = append(parts, ms[i].Format("Jan 2006")+"-"+ms[j].Format("Jan 2006"))
		default:
			parts = append(parts, name(ms[i])+"-"+name(ms[j]))
		}
		i = j + 1
	}
	if len(parts) == 0 {
		return "none"
	}
	return strings.Join(parts, ", ")
}

// fullYear extends the year to December on the assumption that every month
// still to come or to invoice brings what the last invoiced one did, and is
// paid in full on its deadline: the estimate of how the year will end.
func fullYear(c config, months []month, pays []payment, year int) ([]month, []payment, bool) {
	var last month
	for _, m := range months {
		if m.start.Year() == year && m.invoiced {
			last = m
		}
	}
	if !last.invoiced || len(months) == 0 {
		return nil, nil, false
	}
	rate, _ := new(big.Rat).SetString(c.rate)
	rate.Quo(rate, big.NewRat(100, 1))
	ext := slices.Clone(months)
	for m := months[len(months)-1].start.AddDate(0, 1, 0); m.Year() == year; m = m.AddDate(0, 1, 0) {
		next := month{start: m, due: shiftDeadline("PL", m.AddDate(0, 1, 19))}
		switch zusPhase(c, m) {
		case "ulga":
			next.social, next.fpfs = amount{0, official}, amount{0, official}
		default:
			next.social, next.fpfs = socialMonthly(year, zusPhase(c, m) == "preferential", c.chorobowa)
		}
		ext = append(ext, next)
	}
	extPays := slices.Clone(pays)
	for i := range ext {
		m := &ext[i]
		if m.start.Year() != year {
			continue
		}
		if !m.invoiced {
			m.invoiced, m.revenue, m.bracket = true, last.revenue, last.bracket
			m.health = healthMonthly(year, last.bracket)
			m.ryczalt = amount{wholePLN(grosze(new(big.Rat).Mul(ratOf(last.revenue.grosze), rate))), official}
		}
		pay := func(kind string, g int64) {
			if _, _, ok := paidFor(pays, m.start, kind); !ok && g > 0 {
				extPays = append(extPays, payment{paidOn: m.due, period: m.start, kind: kind, grosze: g})
			}
		}
		pay("social", m.social.grosze)
		pay("health", m.health.grosze)
		pay("ryczalt", m.ryczalt.grosze)
	}
	return ext, extPays, true
}

func printYear(w *report, c config, s yearSheet, months []month, pays []payment, today time.Time) {
	pln := func(g int64) string { return plnText(amount{g, official}) }
	short := func(t time.Time) string { return t.Weekday().String()[:3] + " " + t.Format(time.DateOnly) }
	// pad first, colour after, so every number ends in the same column
	// "- ..." and "= ..." rows sit a step in, under the line they work on
	row := func(label, value, note string, loud bool) {
		v := fmt.Sprintf("%13s", value)
		if loud {
			v = paint(w, bold, v)
		}
		indent := ""
		if strings.HasPrefix(label, "- ") || strings.HasPrefix(label, "= ") || strings.HasPrefix(label, "+ ") {
			indent = "  "
		}
		line := "  " + indent + paint(w, cyan, fmt.Sprintf("%-*s", 26-len(indent), label)) + v
		if note != "" {
			line += "  " + paint(w, dim, note)
		}
		w.println(line)
	}
	text := func(label, value string) {
		w.println("  " + paint(w, cyan, fmt.Sprintf("%-26s", label)) + value)
	}
	head := func(title, note string) {
		if note != "" {
			note = "  " + paint(w, dim, note)
		}
		w.println(paint(w, bold, title) + note)
	}

	// the months of the year: all of them, and which are invoiced and paid
	var all, invoiced, ryczPaid, zusPaid []time.Time
	for m := time.Date(s.year, 1, 1, 0, 0, 0, 0, time.UTC); m.Year() == s.year; m = m.AddDate(0, 1, 0) {
		if !m.Before(time.Date(c.businessStart.Year(), c.businessStart.Month(), 1, 0, 0, 0, 0, time.UTC)) {
			all = append(all, m)
		}
	}
	for _, m := range months {
		if m.start.Year() != s.year {
			continue
		}
		if m.invoiced {
			invoiced = append(invoiced, m.start)
		}
		if m.ryczPaid {
			ryczPaid = append(ryczPaid, m.start)
		}
		if m.zusPaid > 0 && !m.zusOpen() {
			zusPaid = append(zusPaid, m.start)
		}
	}
	count := func(ms []time.Time) string {
		return fmt.Sprintf("%-12s %2d of %d months", monthRanges(ms, s.year), len(ms), len(all))
	}
	// ZUS paid during the year, whatever month it was for: what is deducted
	var zusIn []time.Time
	for _, p := range pays {
		if p.paidOn.Year() == s.year && (p.kind == "social" || p.kind == "health") && !slices.Contains(zusIn, p.period) {
			zusIn = append(zusIn, p.period)
		}
	}
	slices.SortFunc(zusIn, func(a, b time.Time) int { return a.Compare(b) })

	w.println(paint(w, bold, fmt.Sprintf("ryczalt year %d", s.year)))
	state := "the year is complete"
	if !s.complete {
		state = "year not over: figures so far, and a year-end estimate"
	}
	w.println(paint(w, dim, fmt.Sprintf("%s | %s%% ryczalt | %s\n", short(today), c.rate, state)))

	head(fmt.Sprintf("Where %d stands", s.year), "")
	text("invoiced", count(invoiced))
	text("ryczalt paid", count(ryczPaid))
	zus := count(zusPaid)
	if extra := slices.DeleteFunc(slices.Clone(zusIn), func(m time.Time) bool { return m.Year() == s.year }); len(extra) > 0 {
		zus += " + " + monthRanges(extra, s.year)
	}
	text("ZUS paid", zus)
	for _, m := range months {
		if m.start.Year() == s.year && (m.zusOpen() || m.ryczOwed() || !m.invoiced) {
			next := "ZUS by " + short(m.due)
			switch {
			case m.ryczOwed():
				next += ", ryczalt by " + short(m.due)
			case !m.invoiced:
				next += ", ryczalt once invoiced"
			}
			if !m.zusOpen() {
				next = strings.Replace(next, "ZUS by "+short(m.due)+", ", "", 1)
			}
			text("next", fmt.Sprintf("%-12s %s", m.start.Format("Jan"), next))
			break
		}
	}

	from := shiftDeadline("PL", time.Date(s.year+1, 2, 15, 0, 0, 0, 0, time.UTC))
	to := shiftDeadline("PL", time.Date(s.year+1, 4, 30, 0, 0, 0, 0, time.UTC))
	w.println()
	head(fmt.Sprintf("PIT-28 for %d", s.year), "filed "+from.Format(time.DateOnly)+" to "+to.Format(time.DateOnly))
	row("revenue", plnText(s.revenue), "invoiced "+monthRanges(invoiced, s.year), false)
	row("- social ZUS paid", "-"+pln(s.socialPaid), "for "+monthRanges(zusIn, s.year)+", paid in "+strconv.Itoa(s.year), false)
	row("- 50% of health ZUS paid", "-"+pln((s.healthPaid+1)/2), "half of "+pln(s.healthPaid)+": ryczalt deducts half", false)
	row("= taxable base", pln(s.base), "rounded to whole PLN", false)
	row("ryczalt "+c.rate+"%", pln(s.tax), "", false)
	row("- ryczalt already paid", "-"+pln(s.advances), monthRanges(ryczPaid, s.year)+", in full", false)
	result := func(label string, r int64) (string, string) {
		if r > 0 {
			return label + "to pay", pln(r)
		}
		return label + "refund", pln(-r)
	}
	l, v := result("= ", s.result)
	if !s.complete {
		l += " so far"
	}
	row(l, v, "", true)

	h := s // the health settlement is for the whole year: estimate it while the year runs
	estimated, like := false, ""
	if !s.complete {
		if ext, extPays, ok := fullYear(c, months, pays, s.year); ok {
			end := time.Date(s.year, 12, 31, 0, 0, 0, 0, time.UTC)
			h, estimated = settleYear(c, ext, extPays, s.year, end), true
			like = invoiced[len(invoiced)-1].Format("Jan")
			l, v := result("", h.result)
			row("year-end estimate", "~"+v, l+" if the rest goes like "+like, true)
		}
	}

	// estimated values carry ~, as forecasts do elsewhere
	est := func(v string) string {
		if estimated {
			return "~" + v
		}
		return v
	}
	due := shiftDeadline("PL", time.Date(s.year+1, 5, 20, 0, 0, 0, 0, time.UTC))
	w.println()
	if estimated {
		w.println(paint(w, bold, fmt.Sprintf("Estimated health settlement for %d", s.year)) + "  " +
			paint(w, dim, "due "+due.Format(time.DateOnly)+" with the ZUS DRA for April") + paint(w, bold+";"+yellow, "*"))
	} else {
		head(fmt.Sprintf("Health settlement for %d", s.year), "due "+due.Format(time.DateOnly)+" with the ZUS DRA for April")
	}
	row("revenue", est(plnText(h.revenue)), "", false)
	row("- social ZUS paid", est("-"+pln(h.socialPaid)), "paid in "+strconv.Itoa(s.year), false)
	row("= yearly base", est(pln(wholePLN(h.healthBase))), "sets the bracket: "+bracketName(h.bracket), false)
	row("due for the year", plnText(h.healthYear), fmt.Sprintf("%d months x %s", h.months, plnText(healthMonthly(s.year, h.bracket))), false)
	billed := healthBilled(months, s.year, estimated)
	var parts, below []string
	for _, b := range billed {
		parts = append(parts, b.months+" "+plnText(amount{b.each, official}))
		if d := healthMonthly(s.year, h.bracket).grosze - b.each; d > 0 {
			below = append(below, fmt.Sprintf("%s: %d x (%s - %s)", b.months, b.count,
				plnText(healthMonthly(s.year, h.bracket)), plnText(amount{b.each, official})))
		}
	}
	row("- billed month by month", "-"+pln(h.healthMonthly), strings.Join(parts, ", "), false)
	switch {
	case h.healthYear.status == unknown:
		row("= difference", "n/a", "no health figures for the year yet", true)
	case h.healthSettle > 0:
		row("= to pay by "+due.Format("2 Jan"), est(pln(h.healthSettle)), strings.Join(below, ", "), true)
	case h.healthSettle < 0:
		row("= overpaid", est(pln(-h.healthSettle)), "refund on request", true)
	default:
		row("= difference", pln(0), "", true)
	}
	if estimated {
		rest := monthRanges(slices.DeleteFunc(slices.Clone(all), func(m time.Time) bool { return slices.Contains(invoiced, m) }), s.year)
		w.println("\n" + paint(w, dim, fmt.Sprintf("* %s assumed like %s; final once the year is invoiced", rest, like)))
	}
}

type billedRun struct {
	months string // e.g. "Jan-Apr"
	count  int
	each   int64 // grosze a month
}

// healthBilled groups the months by the health amount billed, e.g.
// Jan-Apr 498.35 and May-Dec 830.58; toDecember fills the year to its end
// at the last amount.
func healthBilled(months []month, year int, toDecember bool) []billedRun {
	var out []billedRun
	var run []time.Time
	var cur int64 = -1
	flush := func() {
		if len(run) > 0 {
			out = append(out, billedRun{monthRanges(run, year), len(run), cur})
		}
	}
	add := func(m time.Time, g int64) {
		if g != cur {
			flush()
			run, cur = nil, g
		}
		run = append(run, m)
	}
	last := time.Time{}
	for _, m := range months {
		if m.start.Year() == year {
			add(m.start, m.health.grosze)
			last = m.start
		}
	}
	if toDecember && !last.IsZero() {
		for m := last.AddDate(0, 1, 0); m.Year() == year; m = m.AddDate(0, 1, 0) {
			add(m, cur)
		}
	}
	flush()
	return out
}

// healthCliff warns when the year is heading to within a month's revenue
// of a health bracket line, either side: the bracket is set by the whole
// year's base, so one more or one less month moves every month of the year.
func healthCliff(c config, months []month, pays []payment, year int, today time.Time) (finding, bool) {
	if year != today.Year() {
		return finding{}, false
	}
	ext, extPays, ok := fullYear(c, months, pays, year)
	if !ok {
		return finding{}, false
	}
	var monthRevenue int64
	for _, m := range months {
		if m.start.Year() == year && m.invoiced {
			monthRevenue = m.revenue.grosze
		}
	}
	base := settleYear(c, ext, extPays, year, time.Date(year, 12, 31, 0, 0, 0, 0, time.UTC)).healthBase
	pln := func(g int64) string { return strings.TrimSuffix(plnText(amount{wholePLN(g), official}), ".00") }
	for i, line := range healthThresholds {
		name := [...]string{"60k", "300k"}[i]
		switch d := base - line; {
		case d <= 0 && -d < monthRevenue:
			return finding{levelWarn, "health", fmt.Sprintf("year-end base ~%s, %s below the %s line; "+
				"crossing it moves the whole year into the higher band", pln(base), pln(-d), name)}, true
		case d > 0 && d < monthRevenue:
			return finding{levelWarn, "health", fmt.Sprintf("year-end base ~%s, only %s above the %s line; "+
				"the whole year falls in the higher band", pln(base), pln(d), name)}, true
		}
	}
	return finding{}, false
}
