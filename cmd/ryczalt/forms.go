package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

// cmdForms prints what goes into PIT-28(28) and block XII of the ZUS DRA
// for April, field by field, from the year's figures. Field numbers:
// PIT-28(28) as in the MF form definition (crd.e-instytucja.pl, entry 14369),
// DRA block XII as in the Poradnik ZUS DRA of May 2026.
func cmdForms(args []string) error {
	flags := flag.NewFlagSet("forms", flag.ContinueOnError)
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

	year := today.Year()
	if today.Month() <= time.May {
		year--
	}

	switch {
	case flags.NArg() == 1 && yearArg.MatchString(flags.Arg(0)):
		year, _ = strconv.Atoi(flags.Arg(0)) // four digits, checked
	case flags.NArg() > 0:
		return exitError{2, errors.New("usage: ryczalt forms [-c CONFIG] [--today DATE] [YEAR]")}
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

	s := settleYear(c, months, pays, year, today)
	estimated := false
	if !s.complete {
		if ext, extPays, ok := fullYear(c, months, pays, year); ok {
			s = settleYear(c, ext, extPays, year, time.Date(year, 12, 31, 0, 0, 0, 0, time.UTC))
			months, pays, estimated = ext, extPays, true
		}
	}

	w := newReport(os.Stdout)
	printForms(w, c, s, months, pays, today, estimated)

	if len(found) > 0 {
		w.println()
		printFindings(w, found, levelWarn)
	}

	return w.flush()
}

func printForms(w *report, c config, s yearSheet, months []month, pays []payment, today time.Time, estimated bool) {
	pln := func(g int64) string {
		v := plnText(amount{g, official})
		if estimated {
			v = "~" + v
		}
		return v
	}

	field := func(no, label, value, note string) {
		line := "  " + paint(w, cyan, fmt.Sprintf("%-9s", no)) + fmt.Sprintf("%-34s", label) + fmt.Sprintf("%14s", value)
		if note != "" {
			line += "  " + paint(w, dim, note)
		}
		w.println(line)
	}

	state := "the year is complete"
	if estimated {
		state = "year not over: an estimate*"
	}

	w.println(paint(w, bold, fmt.Sprintf("ryczalt forms %d", s.year)))
	w.println(paint(w, dim, fmt.Sprintf("%s %s | %s\n", today.Weekday().String()[:3], today.Format(time.DateOnly), state)))

	from := shiftDeadline("PL", time.Date(s.year+1, 2, 15, 0, 0, 0, 0, time.UTC))
	to := shiftDeadline("PL", time.Date(s.year+1, 4, 30, 0, 0, 0, 0, time.UTC))
	w.println(paint(w, bold, "PIT-28(28)") + "  " + paint(w, dim, fmt.Sprintf("file %s to %s, e-Urzad Skarbowy", from.Format(time.DateOnly), to.Format(time.DateOnly))))

	if c.rate != "12" {
		w.println("  " + tag(w, yellow, "WARN") + " field numbers are mapped for the 12% rate only; use the amounts with care")
	}

	half := (s.healthPaid + 1) / 2
	field("P_4", "tax year", strconv.Itoa(s.year), "")
	field("P_30", "revenue, own business, 12%", pln(s.revenue.grosze), "part D, row 1")
	field("P_57", "revenue together, 12%", pln(s.revenue.grosze), "")
	field("P_63", "revenue together", pln(s.revenue.grosze), "")

	field("P_98", "social contributions paid", pln(s.socialPaid), fmt.Sprintf("paid in %d", s.year))
	field("P_105", "health contributions (art. 11 1a)", pln(half), "50% of "+strings.TrimPrefix(pln(s.healthPaid), "~")+" paid")
	field("P_137", "revenue after deductions, 12%", pln(s.revenue.grosze-s.deductions), "")

	field("P_202", "tax base, 12%", pln(s.base), "whole PLN")
	field("P_217", "ryczalt 12%", pln(s.tax), "")
	field("P_232", "ryczalt due", pln(s.tax), "whole PLN")
	field("P_236", "ryczalt paid", pln(s.advances), "advances for the year's months")

	if s.result > 0 {
		field("P_237", "ryczalt to pay", pln(s.result), "by "+to.Format(time.DateOnly))
	} else {
		field("P_238", "overpayment", pln(-s.result), "refunded after filing")
	}

	// part Q: the ryczałt for each month, as paid
	var q []string
	for mo := time.January; mo <= time.December; mo++ {
		start := time.Date(s.year, mo, 1, 0, 0, 0, 0, time.UTC)
		g, _, _ := paidFor(pays, start, "ryczalt")
		q = append(q, fmt.Sprintf("%s %s", start.Format("Jan"), strings.TrimSuffix(plnText(amount{g, official}), ".00")))
	}

	field("P_239-50", "ryczalt per month (part Q)", "", strings.Join(q, ", "))
	w.println("  " + paint(w, dim, "P_100, P_104, P_107, P_114, P_125, P_177 and P_224-P_230 follow from these; the rest is 0"))

	due := shiftDeadline("PL", time.Date(s.year+1, 5, 20, 0, 0, 0, 0, time.UTC))
	w.println()
	w.println(paint(w, bold, "ZUS DRA for April, block XII") + "  " + paint(w, dim, "health settlement, by "+due.Format(time.DateOnly)))

	if s.months == 0 {
		w.println("  no insured month in the year: no settlement")
	} else {
		field("01", "settlement year", strconv.Itoa(s.year), "")
		field("21", "taxation form: ryczalt", "X", "")
		field("22", "revenue", pln(s.healthBase), "less social ZUS paid")
		field("23", "annual base", pln(s.healthYearBase.grosze), fmt.Sprintf("%d x %s", s.months, plnText(healthBase(s.year, s.bracket))))
		field("24", "annual contribution", pln(s.healthYear.grosze), "9% of field 23")
		field("25", "monthly contributions, sum", pln(s.healthMonthly), "as on the monthly DRAs")

		if s.healthSettle >= 0 {
			field("26", "to pay", pln(s.healthSettle), "")
			field("28", "to pay, together", pln(s.healthSettle), "also into block VI and IX.02; one transfer to your NRS account")
		} else {
			field("27", "overpaid", pln(-s.healthSettle), "")
			field("29", "overpaid, together", pln(-s.healthSettle), "refund on request (RZS-R)")
		}
	}

	if estimated {
		w.println("\n" + paint(w, dim, "* the rest of the year assumed like the last invoiced month; run it again once the year is invoiced"))
	}
}
