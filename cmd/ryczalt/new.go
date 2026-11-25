package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"time"
)

const invoifyDate = "2006-01-02T15:04:05.000Z"

var (
	// the sentences the earlier invoices carry; new rewrites their dates
	// and amounts and leaves everything else as it was
	rateSentence   = regexp.MustCompile(`exchange rate on \d{2}\.\d{2}\.\d{4}: [\d.]+ PLN`)
	periodSentence = regexp.MustCompile(`from \d{2} [A-Z][a-z]+ \d{4} to \d{2} [A-Z][a-z]+ \d{4}`)
)

// latestInvoify finds the newest month folder before folder with a
// readable invoice.json, to copy the next invoice from.
func latestInvoify(root, folder string) (string, error) {
	dirs, err := os.ReadDir(root)
	if err != nil {
		return "", err
	}
	slices.Reverse(dirs)
	for _, d := range dirs {
		if !folderName.MatchString(d.Name()) || d.Name() >= folder {
			continue
		}
		p := filepath.Join(root, d.Name(), "invoice.json")
		if b, err := os.ReadFile(p); err == nil {
			if _, err := readInvoify(b); err == nil {
				return p, nil
			}
		}
	}
	return "", fmt.Errorf("no invoice.json before %s in %s to start from", folder, root)
}

// nextInvoify copies the template's Invoify JSON with what changes from
// month to month filled in. Fields it does not know are kept untouched.
func nextInvoify(tmpl []byte, number string, issued, due, serviceFrom, serviceTo time.Time, rate nbpRate, pln int64) ([]byte, []string, error) {
	dec := json.NewDecoder(bytes.NewReader(tmpl))
	dec.UseNumber() // keep every number exactly as written
	var doc map[string]any
	if err := dec.Decode(&doc); err != nil {
		return nil, nil, err
	}
	d, ok := doc["details"].(map[string]any)
	if !ok {
		return nil, nil, errors.New("template has no details")
	}
	var notes []string
	d["invoiceNumber"] = number
	d["invoiceDate"] = issued.Format(invoifyDate)
	d["dueDate"] = due.Format(invoifyDate)

	published, _ := time.Parse(time.DateOnly, rate.Date)
	rateText := fmt.Sprintf("exchange rate on %s: %d.%02d PLN", published.Format("02.01.2006"), pln/100, pln%100)
	items, _ := d["items"].([]any)
	first, _ := func() (map[string]any, bool) {
		if len(items) == 0 {
			return nil, false
		}
		m, ok := items[0].(map[string]any)
		return m, ok
	}()
	if desc, _ := first["description"].(string); rateSentence.MatchString(desc) {
		first["description"] = rateSentence.ReplaceAllString(desc, rateText)
	} else {
		notes = append(notes, "the first item's description has no exchange rate sentence; add the PLN amount by hand")
	}
	periodText := "from " + serviceFrom.Format("02 January 2006") + " to " + serviceTo.Format("02 January 2006")
	if terms, _ := d["paymentTerms"].(string); periodSentence.MatchString(terms) {
		d["paymentTerms"] = periodSentence.ReplaceAllString(terms, periodText)
	} else {
		notes = append(notes, "payment terms have no service period sentence; add it by hand")
	}

	out, err := json.Marshal(doc)
	return append(out, '\n'), notes, err
}

func cmdNew(args []string) error {
	flags := flag.NewFlagSet("new", flag.ContinueOnError)
	cfgFlag := flags.String("c", "", "config file")
	dateFlag := flags.String("date", "", "issue `DATE` (default the 1st of the next month, or today if later)")
	numberFlag := flags.String("number", "", "invoice `NUMBER` (default the previous one, renumbered)")
	flags.Usage = func() {
		fmt.Fprintln(os.Stderr, "usage: ryczalt new [-c CONFIG] [--date DATE] [--number NUMBER] [MONTH]\n"+
			"MONTH is the month of work (default last month); the invoice goes to the next month's folder.")
	}
	if err := parseFlags(flags, args); err != nil {
		return exitError{2, err}
	}
	today := todayWarsaw()
	service := time.Date(today.Year(), today.Month()-1, 1, 0, 0, 0, 0, time.UTC)
	switch flags.NArg() {
	case 0:
	case 1:
		t, err := time.Parse("2006-01", flags.Arg(0))
		if err != nil {
			return exitError{2, fmt.Errorf("month %q is not like 2026-11", flags.Arg(0))}
		}
		service = t
	default:
		flags.Usage()
		return exitError{2, errors.New("too many arguments")}
	}
	serviceTo := service.AddDate(0, 1, -1)
	issued := service.AddDate(0, 1, 0)
	if today.After(issued) {
		issued = today
	}
	if *dateFlag != "" {
		t, err := time.Parse(time.DateOnly, *dateFlag)
		if err != nil {
			return exitError{2, fmt.Errorf("--date wants a date like 2026-12-01, got %q", *dateFlag)}
		}
		issued = t
	}
	if issued.Before(serviceTo) {
		return fmt.Errorf("issue date %s is before the work ends on %s", issued.Format(time.DateOnly), serviceTo.Format(time.DateOnly))
	}
	folder := issued.Format("2006-01")

	c, err := loadConfig(*cfgFlag)
	if err != nil {
		return err
	}
	tmplPath, err := latestInvoify(c.archive, folder)
	if err != nil {
		return err
	}
	tmpl, err := os.ReadFile(tmplPath)
	if err != nil {
		return err
	}
	prev, _ := readInvoify(tmpl)

	number := *numberFlag
	if number == "" {
		prevFolder := prev.issued.Format("2006-01")
		if !strings.Contains(prev.number, prevFolder) {
			return fmt.Errorf("cannot tell the next number from %s; give --number", prev.number)
		}
		number = strings.Replace(prev.number, prevFolder, folder, 1)
	}
	if invs, _, err := readArchive(c.archive, c.nip); err == nil {
		if i := slices.IndexFunc(invs, func(v invoice) bool { return v.number == number }); i >= 0 {
			return fmt.Errorf("invoice %s already exists (%s); give another --number", number, invs[i].source)
		}
	}
	due := shiftDeadline(c.clientCalendar, issued.AddDate(0, 0, c.paymentDays))

	inv := invoice{number: number, issued: issued, serviceEnd: serviceTo, currency: prev.currency, total: prev.total}
	if inv.currency == "PLN" {
		return errors.New("new handles foreign-currency invoices only so far")
	}
	revenue, rate, err := revenuePLN(inv, c.revenueBasis, cacheDir())
	if err != nil {
		return err
	}
	out, notes, err := nextInvoify(tmpl, number, issued, due, service, serviceTo, rate, revenue)
	if err != nil {
		return fmt.Errorf("%s: %w", tmplPath, err)
	}

	path := filepath.Join(c.archive, folder, "invoice.json")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	if errors.Is(err, fs.ErrExist) {
		return fmt.Errorf("%s already exists, leaving it alone", path)
	}
	if err != nil {
		return err
	}
	if _, err := f.Write(out); err != nil {
		_ = f.Close() // the write error is the one to report
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}

	w := newReport(os.Stdout)
	day := func(t time.Time) string { return t.Weekday().String()[:3] + " " + t.Format(time.DateOnly) }
	label := func(s string) string { return paint(w, dim, fmt.Sprintf("         %-9s", s)) }
	w.printf("%s wrote %s\n", tag(w, green, "OK"), path)
	w.printf("%s %s, issued %s, due %s\n", label("invoice"), paint(w, bold, number), day(issued), day(due))
	w.printf("%s %s to %s\n", label("work"), service.Format(time.DateOnly), serviceTo.Format(time.DateOnly))
	w.printf("%s %s PLN per %s, NBP table %s of %s\n", label("rate"), rate.Mid, inv.currency, rate.Table, rate.Date)
	w.printf("%s %s %s = %s PLN\n", label("revenue"), inv.total.FloatString(2), inv.currency,
		paint(w, bold, plnText(amount{revenue, official})))

	// what the month will cost, with the new invoice in the archive
	invs, _, err := readArchive(c.archive, c.nip)
	if err == nil {
		pays, _, _ := readPayments(c.archive)
		months, _ := buildYear(c, invs, pays, service.Year(), today, cacheDir())
		if i := slices.IndexFunc(months, func(m month) bool { return m.start.Equal(service) }); i >= 0 {
			m := months[i]
			zus := m.zusDue()
			total := amount{zus.grosze + m.ryczalt.grosze, min(zus.status, m.ryczalt.status)}
			w.printf("%s by %s: ryczalt %s*, zus %s, total %s PLN\n", label("to pay"), day(m.due),
				strings.TrimSuffix(plnText(m.ryczalt), ".00"), plnText(zus), paint(w, bold, plnText(total)))
		}
	}
	w.printf("%s copied from %s; review the notes in Invoify before exporting\n",
		tag(w, cyan, "INFO"), filepath.Join(filepath.Base(filepath.Dir(tmplPath)), "invoice.json"))
	for _, n := range notes {
		w.println(tag(w, yellow, "WARN"), n)
	}
	return w.flush()
}
