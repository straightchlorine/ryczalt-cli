package main

import (
	"bytes"
	"cmp"
	"encoding/csv"
	"encoding/xml"
	"errors"
	"flag"
	"fmt"
	"maps"
	"os"
	"slices"
	"strconv"
	"strings"
	"time"
)

// ewidRow is one entry of the ewidencja przychodów (rozporządzenie MF of
// 6 Sep 2025, Dz.U. 2025 poz. 1294; JPK_EWP(4)). One rate per row.
type ewidRow struct {
	lp      int
	entered time.Time // data wpisu
	revenue time.Time // data uzyskania przychodu
	ksef    string    // numer KSeF, "" before KSeF
	doc     string    // numer dowodu
	country string    // country of the buyer's tax id
	buyer   string    // the buyer's tax id
	shown   string    // both, as people write them
	vat     bool      // the buyer's id is a VAT number or NIP
	grosze  int64     // revenue in PLN
	rate    string    // e.g. "12"
	note    string    // uwagi
}

// ewidRows lists the year's revenue in date order. The entry date is the
// day the invoice exists (its issue date, or the revenue date if later):
// within the deadline of the 20th of the next month (§ 6) whenever the
// invoice was issued in time.
func ewidRows(c config, invs []invoice, year int, cacheDir string) ([]ewidRow, []string) {
	var rows []ewidRow
	var notes []string
	for _, inv := range invs {
		d := revenueDate(c.revenueBasis, inv)
		if d.Year() != year {
			continue
		}
		g, r, err := revenuePLN(inv, c.revenueBasis, cacheDir)
		if err != nil {
			notes = append(notes, fmt.Sprintf("%s: %v; left out", inv.number, err))
			continue
		}
		note := ""
		if inv.currency != "PLN" {
			note = fmt.Sprintf(
				"%s %s x %s, NBP %s of %s",
				inv.currency,
				inv.total.FloatString(2),
				r.Mid,
				r.Table,
				r.Date,
			)
		}
		rows = append(rows, ewidRow{
			entered: maxTime(inv.issued, d), revenue: d, ksef: inv.ksefNumber, doc: inv.number,
			country: inv.buyerCode, buyer: inv.buyerID, shown: inv.buyerText(), vat: inv.buyerVAT, grosze: g, rate: c.rate, note: note,
		})
	}
	slices.SortStableFunc(rows, func(a, b ewidRow) int {
		return cmp.Or(a.revenue.Compare(b.revenue), strings.Compare(a.doc, b.doc))
	})
	for i := range rows {
		rows[i].lp = i + 1
	}
	return rows, notes
}

// ewidRates are the rate columns of the register, in its order (art. 12).
var ewidRates = []string{"17", "15", "14", "12.5", "12", "10", "8.5", "5.5", "3"}

func grosz(g int64) string { return fmt.Sprintf("%d.%02d", g/100, g%100) }

// writeEwidCSV writes the register's 17 columns, headed as the regulation
// names them.
func writeEwidCSV(w *report, rows []ewidRow) error {
	var b bytes.Buffer
	cw := csv.NewWriter(&b)
	head := []string{
		"Lp.", "Data wpisu", "Data uzyskania przychodu", "Numer KSeF", "Numer dowodu",
		"Identyfikator podatkowy kontrahenta",
	}
	for _, r := range ewidRates {
		head = append(head, r+"%")
	}
	if err := cw.Write(append(head, "Ogółem", "Uwagi")); err != nil {
		return err
	}
	var total int64
	for _, r := range rows {
		rec := []string{
			strconv.Itoa(r.lp), r.entered.Format(time.DateOnly), r.revenue.Format(time.DateOnly),
			r.ksef, r.doc, r.shown,
		}
		for _, rate := range ewidRates {
			v := ""
			if rate == r.rate {
				v = grosz(r.grosze)
			}
			rec = append(rec, v)
		}
		if err := cw.Write(append(rec, grosz(r.grosze), r.note)); err != nil {
			return err
		}
		total += r.grosze
	}
	cw.Flush()
	if err := cw.Error(); err != nil {
		return err
	}
	w.printf("%s", b.String())
	return nil
}

// jpkNS is the JPK_EWP(4) namespace; etdNS the MF data types it builds on.
const (
	jpkNS = "http://jpk.mf.gov.pl/wzor/2024/10/30/10301/"
	etdNS = "http://crd.gov.pl/xml/schematy/dziedzinowe/mf/2022/01/05/eD/DefinicjeTypy/"
)

// jpkEWP is the year's register as JPK_EWP(4). purpose is CelZlozenia:
// 1 a first file, 2 a correction.
func jpkEWP(c config, rows []ewidRow, year int, purpose int, created time.Time) []byte {
	var b bytes.Buffer
	esc := func(s string) string {
		var e bytes.Buffer
		_ = xml.EscapeText(&e, []byte(s)) // writes to memory
		return e.String()
	}
	el := func(indent, name, value string) {
		if value != "" {
			fmt.Fprintf(&b, "%s<%s>%s</%s>\n", indent, name, esc(value), name)
		}
	}
	b.WriteString(`<?xml version="1.0" encoding="UTF-8"?>` + "\n")
	fmt.Fprintf(&b, "<JPK xmlns=\"%s\" xmlns:etd=\"%s\">\n", jpkNS, etdNS)
	b.WriteString("  <Naglowek>\n")
	b.WriteString(
		`    <KodFormularza kodSystemowy="JPK_EWP (4)" wersjaSchemy="1-0">JPK_EWP</KodFormularza>` + "\n",
	)
	el("    ", "WariantFormularza", "4")
	el("    ", "CelZlozenia", strconv.Itoa(purpose))
	el("    ", "DataWytworzeniaJPK", created.UTC().Format("2006-01-02T15:04:05Z"))
	el("    ", "DataOd", fmt.Sprintf("%d-01-01", year))
	el("    ", "DataDo", fmt.Sprintf("%d-12-31", year))
	el("    ", "KodUrzedu", c.jpkTaxOffice)
	b.WriteString("  </Naglowek>\n")
	b.WriteString("  <Podmiot1 rola=\"Podatnik\">\n    <OsobaFizyczna>\n")
	el("      ", "etd:NIP", c.nip)
	el("      ", "etd:ImiePierwsze", c.jpkFirstName)
	el("      ", "etd:Nazwisko", c.jpkSurname)
	el("      ", "etd:DataUrodzenia", c.jpkBirthDate)
	el("      ", "Email", c.jpkEmail)
	b.WriteString("    </OsobaFizyczna>\n  </Podmiot1>\n")
	var total int64
	for _, r := range rows {
		b.WriteString("  <EWPWiersz>\n")
		el("    ", "K_1", strconv.Itoa(r.lp))
		el("    ", "K_2", r.entered.Format(time.DateOnly))
		el("    ", "K_3", r.revenue.Format(time.DateOnly))
		el("    ", "K_4", r.doc)
		el("    ", "K_5", r.ksef)
		if r.buyer != "" {
			el("    ", "K_6", r.country)
			el("    ", "K_7", r.buyer)
		}
		el("    ", "K_8", grosz(r.grosze))
		el("    ", "K_9", r.rate)
		el("    ", "K_10", r.note)
		b.WriteString("  </EWPWiersz>\n")
		total += r.grosze
	}
	b.WriteString("  <EWPCtrl>\n")
	el("    ", "LiczbaWierszy", strconv.Itoa(len(rows)))
	el("    ", "SumaPrzychodow", grosz(total))
	b.WriteString("  </EWPCtrl>\n</JPK>\n")
	return b.Bytes()
}

func cmdEwid(args []string) error {
	flags := flag.NewFlagSet("ewid", flag.ContinueOnError)
	cfgFlag := flags.String("c", "", "config file")
	asCSV := flags.Bool("csv", false, "write the register as CSV")
	asJPK := flags.Bool("jpk", false, "write the register as a JPK_EWP(4) file")
	correction := flags.Bool(
		"correction",
		false,
		"with --jpk: a correction (CelZlozenia 2) of a file already sent",
	)
	flags.Usage = func() {
		fmt.Fprintln(
			os.Stderr,
			"usage: ryczalt ewid [-c CONFIG] [--csv | --jpk [--correction]] [YEAR]",
		)
	}
	if err := parseFlags(flags, args); err != nil {
		return exitError{2, err}
	}
	today := todayWarsaw()
	year := today.Year()
	if today.Month() <= time.May {
		year-- // until the PIT-28 and JPK deadlines are past, last year
	}
	switch {
	case flags.NArg() == 1 && yearArg.MatchString(flags.Arg(0)):
		year, _ = strconv.Atoi(flags.Arg(0)) // four digits, checked
	case flags.NArg() > 0, *asCSV && *asJPK, *correction && !*asJPK:
		flags.Usage()
		return exitError{2, errors.New("wrong arguments")}
	}
	c, err := loadConfig(*cfgFlag)
	if err != nil {
		return err
	}
	if *asJPK {
		if missing := c.jpkMissing(); len(missing) > 0 {
			return exitError{
				2,
				fmt.Errorf("--jpk needs [jpk] %s in the config", strings.Join(missing, ", ")),
			}
		}
	}
	invs, found, err := readArchive(c.archive, c.nip)
	if err != nil {
		return err
	}
	rows, notes := ewidRows(c, invs, year, cacheDir())
	found = append(found, asFindings("rates", notes)...)

	out := newReport(os.Stdout)
	switch {
	case *asJPK:
		purpose := 1
		if *correction {
			purpose = 2
		}
		out.printf("%s", jpkEWP(c, rows, year, purpose, time.Now()))
	case *asCSV:
		if err := writeEwidCSV(out, rows); err != nil {
			return err
		}
	default:
		printEwid(out, c, rows, year)
	}
	if err := out.flush(); err != nil {
		return err
	}
	if slices.ContainsFunc(found, func(f finding) bool { return f.level == levelWarn }) {
		errs := newReport(os.Stderr) // never into the CSV or the XML
		printFindings(errs, found, levelWarn)
		return errs.flush()
	}
	return nil
}

// entryDeadline is the last day to enter a revenue: the 20th of the
// month after it (§ 6 of the regulation), moved past days off.
func entryDeadline(revenue time.Time) time.Time {
	return shiftDeadline(
		"PL",
		time.Date(revenue.Year(), revenue.Month()+1, 20, 0, 0, 0, 0, time.UTC),
	)
}

func printEwid(w *report, c config, rows []ewidRow, year int) {
	w.println(paint(w, bold, fmt.Sprintf("ryczalt ewid %d", year)))
	w.println(
		paint(
			w,
			dim,
			fmt.Sprintf(
				"ewidencja przychodow, %d entries; --csv for a spreadsheet, --jpk for JPK_EWP(4)\n",
				len(rows),
			),
		),
	)
	if len(rows) == 0 {
		w.println("No revenue in the year.")
		return
	}
	const colLp, colEntered, colDoc, colKSeF, colBuyer = 0, 1, 3, 4, 5
	table := [][]string{
		{"Lp", "Entered", "Revenue", "Document", "KSeF number", "Buyer", c.rate + "% PLN"},
	}
	var total int64
	late, noVAT := map[int]bool{}, map[int]bool{}
	for i, r := range rows {
		table = append(table, []string{
			strconv.Itoa(r.lp), r.entered.Format(time.DateOnly), r.revenue.Format(time.DateOnly),
			r.doc, cmp.Or(r.ksef, "-"), cmp.Or(r.shown, "-"), plnText(amount{r.grosze, official}),
		})
		total += r.grosze
		late[i+1] = r.entered.After(entryDeadline(r.revenue))
		noVAT[i+1] = r.buyer != "" && !r.vat
	}
	table = append(table, []string{"", "", "", "", "", "total", plnText(amount{total, official})})
	right := []bool{true, false, false, false, false, false, true}
	printTable(w, table, right, func(row, col int) string {
		switch {
		case row == 0 || row == len(table)-1:
			return bold
		case table[row][col] == "-":
			return dim
		case col == colEntered && late[row], col == colBuyer && noVAT[row]:
			return yellow
		case col == colLp, col == colEntered, col == colKSeF:
			return dim
		case col == colDoc:
			return cyan
		}
		return ""
	})
	var legend []string
	if slices.Contains(slices.Collect(maps.Values(late)), true) {
		legend = append(legend, "entered after the 20th of the next month (par. 6)")
	}
	if slices.Contains(slices.Collect(maps.Values(noVAT)), true) {
		legend = append(legend, "buyer named without a VAT number (ryczalt check)")
	}
	if len(legend) > 0 {
		w.println(
			"\n" + paint(w, yellow, "yellow") + paint(w, dim, ": "+strings.Join(legend, "; ")),
		)
	}
}
