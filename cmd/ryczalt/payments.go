package main

import (
	"crypto/rand"
	"encoding/csv"
	"encoding/hex"
	"errors"
	"flag"
	"fmt"
	"io/fs"
	"math/big"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"time"
)

// A payment is one transfer, or the part of one, for one month and one
// kind. They live in <archive>/YYYY/payments.<device>.csv, by the year
// paid: append-only, one file per device so Syncthing never sees two
// machines write the same file. A mistake is cancelled by a void row,
// never edited away.
type payment struct {
	id     string
	paidOn time.Time
	period time.Time // the month it is for, first day
	kind   string    // social | health | fpfs | ryczalt | void
	grosze int64
	voids  string // for kind void: the id it cancels
	source string
}

var payKinds = []string{"social", "health", "fpfs", "ryczalt"}

const paymentsHeader = "id,paid_on,period,kind,amount,note"

var (
	periodArg    = regexp.MustCompile(`^(\d{4}-\d{2})(?:\.\.(\d{4}-\d{2}))?$`)
	paymentsName = regexp.MustCompile(`^payments\.[A-Za-z0-9-]+\.csv$`)
	yearFolder   = regexp.MustCompile(`^\d{4}$`)
)

// readPayments reads every payments file in the archive and drops voided rows.
// A Syncthing conflict copy or a duplicate id is a warning.
func readPayments(root string) ([]payment, []string, error) {
	years, err := os.ReadDir(root)
	if err != nil {
		return nil, nil, err
	}
	var all []payment
	var warns []string
	for _, y := range years {
		if !y.IsDir() || !yearFolder.MatchString(y.Name()) {
			continue
		}
		files, err := os.ReadDir(filepath.Join(root, y.Name()))
		if err != nil {
			return nil, nil, err
		}
		for _, f := range files {
			rel := filepath.Join(y.Name(), f.Name())
			if strings.HasPrefix(f.Name(), "payments.") && !paymentsName.MatchString(f.Name()) {
				warns = append(warns, rel+": looks like a sync conflict copy; merge it by hand and delete it")
				continue
			}
			if !paymentsName.MatchString(f.Name()) {
				continue
			}
			ps, err := readPaymentsFile(filepath.Join(root, rel), rel)
			if err != nil {
				return nil, nil, err
			}
			all = append(all, ps...)
		}
	}

	seen := map[string]string{}
	voided := map[string]bool{}
	for _, p := range all {
		if prev, ok := seen[p.id]; ok {
			warns = append(warns, fmt.Sprintf("payment id %s is in both %s and %s; counted once", p.id, prev, p.source))
		}
		seen[p.id] = p.source
		if p.kind == "void" {
			voided[p.voids] = true
		}
	}
	var out []payment
	done := map[string]bool{}
	for _, p := range all {
		if p.kind != "void" && !voided[p.id] && !done[p.id] {
			out = append(out, p)
			done[p.id] = true
		}
	}
	slices.SortFunc(out, func(a, b payment) int { return a.paidOn.Compare(b.paidOn) })
	return out, warns, nil
}

func readPaymentsFile(path, rel string) ([]payment, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	rows, err := csv.NewReader(f).ReadAll()
	if err != nil {
		return nil, fmt.Errorf("%s: %w", rel, err)
	}
	if len(rows) == 0 || strings.Join(rows[0], ",") != paymentsHeader {
		return nil, fmt.Errorf("%s: first line must be %s", rel, paymentsHeader)
	}
	var out []payment
	for i, r := range rows[1:] {
		bad := func(what string) error { return fmt.Errorf("%s line %d: %s", rel, i+2, what) }
		paidOn, err1 := time.Parse(time.DateOnly, r[1])
		period, err2 := time.Parse("2006-01", r[2])
		if r[0] == "" || err1 != nil || err2 != nil {
			return nil, bad("needs an id, a paid_on date and a period like 2026-10")
		}
		p := payment{id: r[0], paidOn: paidOn, period: period, kind: r[3], source: rel}
		switch {
		case p.kind == "void":
			p.voids = r[5]
		case slices.Contains(payKinds, p.kind):
			amt, ok := new(big.Rat).SetString(r[4])
			if !ok || amt.Sign() < 0 {
				return nil, bad(fmt.Sprintf("amount %q is not a positive number", r[4]))
			}
			p.grosze = grosze(amt)
		default:
			return nil, bad(fmt.Sprintf("kind %q is not one of %s or void", p.kind, strings.Join(payKinds, ", ")))
		}
		out = append(out, p)
	}
	return out, nil
}

// appendPayments writes rows to this device's file for the year of paidOn,
// with one write, so a crash never leaves half a line.
func appendPayments(root string, ps []payment) (string, error) {
	host, _ := os.Hostname()
	host = regexp.MustCompile(`[^A-Za-z0-9-]`).ReplaceAllString(host, "-")
	if host == "" {
		host = "local"
	}
	rel := filepath.Join(ps[0].paidOn.Format("2006"), "payments."+host+".csv")
	path := filepath.Join(root, rel)
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return "", err
	}
	var b strings.Builder
	if _, err := os.Stat(path); errors.Is(err, fs.ErrNotExist) {
		b.WriteString(paymentsHeader + "\n")
	}
	w := csv.NewWriter(&b)
	for _, p := range ps {
		amount := ""
		if p.kind != "void" {
			amount = fmt.Sprintf("%d.%02d", p.grosze/100, p.grosze%100)
		}
		w.Write([]string{p.id, p.paidOn.Format(time.DateOnly), p.period.Format("2006-01"), p.kind, amount, p.voids})
	}
	w.Flush()
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_APPEND|os.O_CREATE, 0o600)
	if err != nil {
		return "", err
	}
	if _, err := f.WriteString(b.String()); err != nil {
		f.Close()
		return "", err
	}
	return rel, f.Close()
}

func newPaymentID(paidOn time.Time) string {
	b := make([]byte, 3)
	rand.Read(b)
	return paidOn.Format("20060102") + "-" + hex.EncodeToString(b)
}

// paidFor sums the payments of one kind for one month.
func paidFor(ps []payment, period time.Time, kind string) (sum int64, last time.Time, ok bool) {
	for _, p := range ps {
		if p.period.Equal(period) && p.kind == kind {
			sum, ok = sum+p.grosze, true
			if p.paidOn.After(last) {
				last = p.paidOn
			}
		}
	}
	return sum, last, ok
}

const paidUsage = `usage: ryczalt paid [-c CONFIG] [--on DATE | --on-due] PERIOD [KIND [AMOUNT]]
       ryczalt paid [-c CONFIG] --list [YEAR]
PERIOD is a month (2026-10) or a range (2026-01..2026-10). KIND is zus
(social, health and FP/FS at the amounts due), social, health, fpfs or
ryczalt; none means zus and ryczalt. AMOUNT overrides the amount due.`

func cmdPaid(args []string) error {
	flags := flag.NewFlagSet("paid", flag.ContinueOnError)
	cfgFlag := flags.String("c", "", "config file")
	onFlag := flags.String("on", "", "date paid (default today)")
	onDue := flags.Bool("on-due", false, "record each month as paid on its deadline")
	list := flags.Bool("list", false, "list the payments of a year")
	flags.Usage = func() { fmt.Fprintln(os.Stderr, paidUsage) }
	if err := flags.Parse(args); err != nil {
		return exitError{2, err}
	}
	c, err := loadConfig(*cfgFlag)
	if err != nil {
		return err
	}
	ps, warns, err := readPayments(c.archive)
	if err != nil {
		return err
	}
	for _, s := range warns {
		fmt.Println(tag(os.Stdout, yellow, "WARN"), s)
	}
	if *list {
		return listPayments(ps, flags.Args())
	}

	a := flags.Args()
	if len(a) == 0 || len(a) > 3 || (*onFlag != "" && *onDue) {
		return exitError{2, errors.New(paidUsage)}
	}
	m := periodArg.FindStringSubmatch(a[0])
	if m == nil {
		return exitError{2, fmt.Errorf("period %q is not a month like 2026-10 or a range like 2026-01..2026-10", a[0])}
	}
	from, _ := time.Parse("2006-01", m[1])
	to := from
	if m[2] != "" {
		to, _ = time.Parse("2006-01", m[2])
	}
	if to.Before(from) || from.Year() != to.Year() {
		return exitError{2, errors.New("a range must run forward within one year")}
	}
	kind := ""
	if len(a) > 1 {
		kind = a[1]
		if kind != "zus" && !slices.Contains(payKinds, kind) {
			return exitError{2, fmt.Errorf("kind %q is not zus, %s", kind, strings.Join(payKinds, ", "))}
		}
	}
	var override int64 = -1
	if len(a) == 3 {
		r, ok := new(big.Rat).SetString(strings.ReplaceAll(a[2], ",", "."))
		if !ok || r.Sign() <= 0 || kind == "zus" || kind == "" || from != to {
			return exitError{2, errors.New("an AMOUNT needs one month and one kind other than zus " +
				"(record social and health separately)")}
		}
		override = grosze(r)
	}
	today := todayWarsaw()
	paidOn := today
	if *onFlag != "" {
		if paidOn, err = time.Parse(time.DateOnly, *onFlag); err != nil {
			return exitError{2, fmt.Errorf("--on wants a date like 2026-11-20, got %q", *onFlag)}
		}
	}

	invs, _, err := readArchive(c.archive, c.nip)
	if err != nil {
		return err
	}
	months, notes := buildYear(c, invs, nil, from.Year(), today, cacheDir())
	for _, n := range notes {
		fmt.Println(tag(os.Stdout, yellow, "WARN"), n)
	}
	var rows []payment
	for p := from; !p.After(to); p = p.AddDate(0, 1, 0) {
		i := slices.IndexFunc(months, func(m month) bool { return m.start.Equal(p) })
		if i < 0 {
			return fmt.Errorf("%s is not a month of business up to today", p.Format("2006-01"))
		}
		mo := months[i]
		on := paidOn
		if *onDue {
			on = mo.due
		}
		due := map[string]amount{"social": mo.social, "health": mo.health, "fpfs": mo.fpfs, "ryczalt": mo.ryczalt}
		kinds := []string{kind}
		switch kind {
		case "":
			kinds = []string{"social", "health", "fpfs", "ryczalt"}
		case "zus":
			kinds = []string{"social", "health", "fpfs"}
		}
		for _, k := range kinds {
			g := override
			if g < 0 {
				if due[k].status == unknown {
					if kind == "" && k == "ryczalt" && !mo.invoiced {
						continue // ZUS only for a month not yet invoiced
					}
					return fmt.Errorf("%s %s: the amount due is not known; give AMOUNT", p.Format("2006-01"), k)
				}
				g = due[k].grosze
			}
			if g == 0 {
				continue // e.g. FP/FS outside the full phase
			}
			if _, _, ok := paidFor(ps, p, k); ok && override < 0 {
				return fmt.Errorf("%s %s is already recorded; see ryczalt paid --list, "+
					"void it first, or give AMOUNT for a further payment", p.Format("2006-01"), k)
			}
			rows = append(rows, payment{id: newPaymentID(on), paidOn: on, period: p, kind: k, grosze: g})
		}
	}
	if len(rows) == 0 {
		return errors.New("nothing to record")
	}
	files := map[string]bool{}
	for _, r := range rows {
		rel, err := appendPayments(c.archive, []payment{r})
		if err != nil {
			return err
		}
		files[rel] = true
		fmt.Printf("%s %s  %s %-7s %10s PLN  paid %s\n", tag(os.Stdout, green, "OK"), r.id,
			r.period.Format("2006-01"), r.kind, plnText(amount{r.grosze, official}), r.paidOn.Format(time.DateOnly))
	}
	for f := range files {
		fmt.Printf("%s recorded in %s\n", tag(os.Stdout, cyan, "INFO"), filepath.Join(c.archive, f))
	}
	return nil
}

func listPayments(ps []payment, args []string) error {
	year := todayWarsaw().Year()
	if len(args) == 1 && yearArg.MatchString(args[0]) {
		fmt.Sscan(args[0], &year)
	} else if len(args) > 0 {
		return exitError{2, errors.New(paidUsage)}
	}
	rows := [][]string{{"Id", "Paid on", "For", "Kind", "PLN"}}
	for _, p := range ps {
		if p.paidOn.Year() == year || p.period.Year() == year {
			rows = append(rows, []string{
				p.id, p.paidOn.Format(time.DateOnly), p.period.Format("2006-01"),
				p.kind, plnText(amount{p.grosze, official}),
			})
		}
	}
	if len(rows) == 1 {
		fmt.Printf("No payments recorded for %d.\n", year)
		return nil
	}
	printTable(os.Stdout, rows, []bool{false, false, false, false, true}, func(r, col int) string {
		if r == 0 {
			return bold
		}
		if col == 0 {
			return dim
		}
		return ""
	})
	return nil
}

func cmdVoid(args []string) error {
	flags := flag.NewFlagSet("void", flag.ContinueOnError)
	cfgFlag := flags.String("c", "", "config file")
	if err := flags.Parse(args); err != nil {
		return exitError{2, err}
	}
	if flags.NArg() == 0 {
		return exitError{2, errors.New("usage: ryczalt void [-c CONFIG] ID...  (ids from ryczalt paid --list)")}
	}
	c, err := loadConfig(*cfgFlag)
	if err != nil {
		return err
	}
	ps, _, err := readPayments(c.archive)
	if err != nil {
		return err
	}
	today := todayWarsaw()
	var rows []payment
	for _, id := range flags.Args() {
		i := slices.IndexFunc(ps, func(p payment) bool { return p.id == id })
		if i < 0 {
			return fmt.Errorf("no payment %s (or it is voided already)", id)
		}
		rows = append(rows, payment{id: newPaymentID(today), paidOn: today, period: ps[i].period, kind: "void", voids: id})
	}
	rel, err := appendPayments(c.archive, rows)
	if err != nil {
		return err
	}
	for _, r := range rows {
		fmt.Printf("%s voided %s (%s)\n", tag(os.Stdout, green, "OK"), r.voids, filepath.Join(c.archive, rel))
	}
	return nil
}

func todayWarsaw() time.Time {
	y, m, d := time.Now().In(warsaw).Date()
	return time.Date(y, m, d, 0, 0, 0, 0, time.UTC)
}

func cacheDir() string { return xdgPath("XDG_CACHE_HOME", ".cache", "ryczalt") }
