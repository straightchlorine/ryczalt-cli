package main

import (
	"bufio"
	"bytes"
	"cmp"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"
)

// config is ryczalt.conf. Keys arrive with the code that uses them, and
// an unknown key is an error, so a typo never passes silently.
type config struct {
	archive           string    // folder holding the YYYY-MM invoice folders
	nip               string    // the seller's NIP; invoices from anyone else are not revenue
	businessStart     time.Time // first month of business
	suspendedFrom     time.Time // first month the business is suspended (zawieszenie); zero = never
	suspendedUntil    time.Time // last suspended month; zero = until further notice
	rate              string    // ryczałt rate in percent, e.g. "12"
	revenueBasis      string    // period_end | service, see ryczalt.conf.example
	ulgaUntil         time.Time // last month of ulga na start; zero = never had it
	preferentialUntil time.Time // last month of preferential ZUS; zero = never had it
	chorobowa         bool      // voluntary sickness insurance
	paymentDays       int       // days the client has to pay
	clientCalendar    string    // whose holidays move the client's deadline, see validCalendar
	ksefBin           string    // ksef-cli binary; "" = ksef from PATH
	ksefConfig        string    // KSEF_CONFIG for it; "" = ksef-cli's own default
	r2Bucket          string    // off-site copy; "" = none
	r2Profile         string
	r2Endpoint        string
	jpkFirstName      string // for JPK_EWP only
	jpkSurname        string
	jpkBirthDate      string
	jpkTaxOffice      string // kod urzędu skarbowego, 4 digits
	jpkEmail          string
}

// jpkMissing names the [jpk] keys a JPK_EWP file needs and lacks.
func (c config) jpkMissing() []string {
	var m []string
	for k, v := range map[string]string{
		"first_name": c.jpkFirstName, "surname": c.jpkSurname,
		"birth_date": c.jpkBirthDate, "tax_office": c.jpkTaxOffice,
	} {
		if v == "" {
			m = append(m, k)
		}
	}
	slices.Sort(m)
	return m
}

var configKeys = []string{
	"archive",
	"tax.nip", "tax.form", "tax.payment", "tax.vat_status", "tax.business_start", "tax.suspended_from", "tax.suspended_until",
	"tax.rate", "tax.revenue_basis",
	"zus.ulga_until", "zus.preferential_until", "zus.chorobowa",
	"client.payment_days", "client.calendar",
	"ksef.bin", "ksef.config",
	"r2.bucket", "r2.profile", "r2.endpoint",
	"jpk.first_name", "jpk.surname", "jpk.birth_date", "jpk.tax_office", "jpk.email",
}

// errUnsupported marks a setup ryczalt does not handle; commands exit 3 on it.
var errUnsupported = errors.New("unsupported setup")

// parseConfig reads key=value lines with shell-style values under optional
// [section] headers. A malformed line stops it; problems with values are
// all reported together.
func parseConfig(r io.Reader) (config, error) {
	vals := map[string]string{}
	section := ""
	sc := bufio.NewScanner(r)
	for n := 1; sc.Scan(); n++ {
		line := strings.TrimSpace(sc.Text())
		if n == 1 {
			line = strings.TrimPrefix(line, "\ufeff") // byte order mark some editors add
		}
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if head := parseValue(line); strings.HasPrefix(head, "[") && strings.HasSuffix(head, "]") {
			section = strings.TrimSpace(head[1 : len(head)-1])
			if !slices.ContainsFunc(
				configKeys,
				func(k string) bool { return strings.HasPrefix(k, section+".") },
			) {
				return config{}, fmt.Errorf("line %d: unknown section [%s]", n, section)
			}
			continue
		}
		k, raw, ok := strings.Cut(line, "=")
		if !ok {
			return config{}, fmt.Errorf("line %d: not a key=value line: %q", n, line)
		}
		key := strings.TrimSpace(k)
		if section != "" {
			key = section + "." + key
		}
		if !slices.Contains(configKeys, key) {
			return config{}, fmt.Errorf("line %d: unknown key %s", n, key)
		}
		if _, dup := vals[key]; dup {
			return config{}, fmt.Errorf("line %d: %s is set twice", n, key)
		}
		vals[key] = parseValue(raw)
	}
	if err := sc.Err(); err != nil {
		return config{}, err
	}

	var errs []error
	bad := func(key, msg string) { errs = append(errs, fmt.Errorf("%s: %s", key, msg)) }
	required := func(key string) string {
		v := vals[key]
		if v == "" {
			bad(key, "required")
		}
		return v
	}
	only := func(key, want string) {
		if v := required(key); v != "" && v != want {
			errs = append(
				errs,
				fmt.Errorf("%w: %s=%s, only %s is supported", errUnsupported, key, v, want),
			)
		}
	}
	oneOf := func(key string, allowed ...string) string {
		v := required(key)
		if v != "" && !slices.Contains(allowed, v) {
			bad(key, fmt.Sprintf("%q is not one of %s", v, strings.Join(allowed, " | ")))
		}
		return v
	}
	month := func(key string) time.Time {
		v := vals[key]
		if v == "" {
			return time.Time{}
		}
		t, err := time.Parse("2006-01", v)
		if err != nil {
			bad(key, fmt.Sprintf("%q is not a month like 2026-01", v))
		}
		return t
	}

	var c config
	c.archive = expandHome(required("archive"))
	if c.archive != "" && !filepath.IsAbs(c.archive) {
		bad("archive", fmt.Sprintf("%q must be an absolute path or start with ~/", c.archive))
	}
	if c.nip = required("tax.nip"); c.nip != "" && !validNIP(c.nip) {
		bad("tax.nip", fmt.Sprintf("%q is not a valid NIP (10 digits, no dashes)", c.nip))
	}
	only("tax.form", "ryczalt")
	only("tax.payment", "monthly")
	only("tax.vat_status", "exempt")
	required("tax.business_start")
	c.businessStart = month("tax.business_start")
	c.suspendedFrom, c.suspendedUntil = month("tax.suspended_from"), month("tax.suspended_until")
	switch {
	case !c.suspendedUntil.IsZero() && c.suspendedFrom.IsZero():
		bad("tax.suspended_until", "needs tax.suspended_from")
	case !c.suspendedUntil.IsZero() && c.suspendedUntil.Before(c.suspendedFrom):
		bad("tax.suspended_until", "before tax.suspended_from")
	case !c.suspendedFrom.IsZero() && !c.businessStart.IsZero() && c.suspendedFrom.Before(c.businessStart):
		bad("tax.suspended_from", "before tax.business_start")
	}
	// the rates of art. 12 ust. 1 ustawy o zryczałtowanym podatku
	c.rate = oneOf("tax.rate", "17", "15", "14", "12.5", "12", "10", "8.5", "5.5", "3")
	c.revenueBasis = oneOf("tax.revenue_basis", "period_end", "service")
	c.ulgaUntil = month("zus.ulga_until")
	c.preferentialUntil = month("zus.preferential_until")
	for _, p := range []struct {
		key string
		t   time.Time
	}{{"zus.ulga_until", c.ulgaUntil}, {"zus.preferential_until", c.preferentialUntil}} {
		if !p.t.IsZero() && !c.businessStart.IsZero() && p.t.Before(c.businessStart) {
			bad(p.key, "before tax.business_start")
		}
	}
	if !c.ulgaUntil.IsZero() && !c.preferentialUntil.IsZero() &&
		!c.preferentialUntil.After(c.ulgaUntil) {
		bad("zus.preferential_until", "must come after zus.ulga_until")
	}
	c.chorobowa = oneOf("zus.chorobowa", "yes", "no") == "yes"
	if v := required("client.payment_days"); v != "" {
		if n, err := strconv.Atoi(v); err != nil || n < 0 || n > 365 {
			bad("client.payment_days", fmt.Sprintf("%q is not a number of days", v))
		} else {
			c.paymentDays = n
		}
	}
	if c.clientCalendar = strings.Join(strings.Fields(required("client.calendar")), " "); c.clientCalendar != "" &&
		!validCalendar(c.clientCalendar) {
		bad(
			"client.calendar",
			fmt.Sprintf(
				"%q is not PL or DE-<state> (e.g. DE-BY), with optional +MM-DD days",
				c.clientCalendar,
			),
		)
	}
	c.ksefBin = expandHome(vals["ksef.bin"])
	c.ksefConfig = expandHome(vals["ksef.config"])
	c.r2Bucket, c.r2Profile, c.r2Endpoint = vals["r2.bucket"], vals["r2.profile"], vals["r2.endpoint"]
	c.jpkFirstName, c.jpkSurname, c.jpkEmail = vals["jpk.first_name"], vals["jpk.surname"], vals["jpk.email"]
	if c.jpkBirthDate = vals["jpk.birth_date"]; c.jpkBirthDate != "" {
		if _, err := time.Parse(time.DateOnly, c.jpkBirthDate); err != nil {
			bad("jpk.birth_date", fmt.Sprintf("%q is not a date like 1990-05-17", c.jpkBirthDate))
		}
	}
	if c.jpkTaxOffice = vals["jpk.tax_office"]; c.jpkTaxOffice != "" &&
		!regexp.MustCompile(`^\d{4}$`).MatchString(c.jpkTaxOffice) {
		bad(
			"jpk.tax_office",
			fmt.Sprintf(
				"%q is not a 4-digit tax office code (kod urzedu skarbowego)",
				c.jpkTaxOffice,
			),
		)
	}
	if c.r2Bucket == "" && (c.r2Profile != "" || c.r2Endpoint != "") {
		bad("r2.bucket", "required when [r2] has a profile or an endpoint")
	}

	return c, errors.Join(errs...)
}

// parseValue reads a value the way the shell would.
func parseValue(s string) string {
	if q := strings.TrimLeft(s, " \t"); len(q) != len(s) && strings.HasPrefix(q, "#") {
		return ""
	}
	s = strings.TrimSpace(s)
	if len(s) >= 2 && (s[0] == '"' || s[0] == '\'') {
		if end := strings.IndexByte(s[1:], s[0]); end >= 0 {
			return s[1 : 1+end]
		}
	}
	for i := 1; i < len(s); i++ {
		if s[i] == '#' && (s[i-1] == ' ' || s[i-1] == '\t') {
			s = s[:i]
			break
		}
	}
	return strings.TrimSpace(s)
}

func expandHome(path string) string {
	if path == "~" || strings.HasPrefix(path, "~/") {
		if home, err := os.UserHomeDir(); err == nil {
			return filepath.Join(home, path[1:])
		}
	}
	return path
}

// validNIP checks the 10 digits and the check digit of a Polish tax id
// (copied from ksef-cli, cmd/ksef/invoice.go).
func validNIP(nip string) bool {
	if len(nip) != 10 {
		return false
	}
	sum := 0
	for i := range 10 {
		if nip[i] < '0' || nip[i] > '9' {
			return false
		}
		if i < 9 {
			sum += []int{6, 5, 7, 2, 3, 4, 5, 6, 7}[i] * int(nip[i]-'0')
		}
	}
	return sum%11 == int(nip[9]-'0')
}

// exitError carries the exit code a failure should end the program with:
// 2 for usage and config errors, 3 for an unsupported setup.
type exitError struct {
	code int
	err  error
}

func (e exitError) Error() string { return e.err.Error() }

func (e exitError) Unwrap() error { return e.err }

// configPath is -c if given, else $RYCZALT_CONFIG, else
// $XDG_CONFIG_HOME/ryczalt.conf (default ~/.config/ryczalt.conf).
func configPath(flag string) string {
	if p := cmp.Or(flag, os.Getenv("RYCZALT_CONFIG")); p != "" {
		return p
	}
	return xdgPath("XDG_CONFIG_HOME", ".config", "ryczalt.conf")
}

func loadConfig(flag string) (config, error) {
	path := configPath(flag)
	b, err := os.ReadFile(path)
	if err != nil {
		return config{}, exitError{2, fmt.Errorf("%w; start from ryczalt.conf.example", err)}
	}
	c, err := parseConfig(bytes.NewReader(b))
	if err != nil {
		code := 2
		if errors.Is(err, errUnsupported) {
			code = 3
		}
		return c, exitError{code, fmt.Errorf("%s:\n%w", path, err)}
	}
	if st, err := os.Stat(c.archive); err != nil || !st.IsDir() {
		return c, exitError{2, fmt.Errorf("%s: archive %s is not a folder", path, c.archive)}
	}
	return c, nil
}

// xdgPath is $envVar/name, or ~/fallback/name when the variable is unset
// (copied from ksef-cli, cmd/ksef/config.go).
func xdgPath(envVar, fallback, name string) string {
	dir := os.Getenv(envVar)
	if dir == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return name
		}
		dir = filepath.Join(home, fallback)
	}
	return filepath.Join(dir, name)
}

// suspended reports whether month m (its first day) is a month of the
// business suspended in full: no ZUS, no health contribution, no invoice.
// NOTE: whole months only
func (c config) suspended(m time.Time) bool {
	return !c.suspendedFrom.IsZero() && !m.Before(c.suspendedFrom) &&
		(c.suspendedUntil.IsZero() || !m.After(c.suspendedUntil))
}
