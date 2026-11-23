package main

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"
)

// config is ryczalt.conf. Keys arrive with the code that uses them, and
// an unknown key is an error, so a typo never passes silently.
type config struct {
	archive           string    // folder holding the YYYY-MM invoice folders
	businessStart     time.Time // first month of business
	revenueBasis      string    // period_end | service, see ryczalt.conf.example
	methodConfirmedOn string    // when an accountant confirmed revenueBasis; "" = not yet
	ulgaUntil         time.Time // last month of ulga na start; zero = never had it
	preferentialUntil time.Time // last month of preferential ZUS; zero = never had it
	chorobowa         bool      // voluntary sickness insurance
}

var configKeys = []string{
	"archive",
	"tax.form", "tax.payment", "tax.vat_status", "tax.business_start",
	"tax.revenue_basis", "tax.method_confirmed_on",
	"zus.ulga_until", "zus.preferential_until", "zus.chorobowa",
}

// errUnsupported marks a setup ryczalt does not handle; it exits 3.
var errUnsupported = errors.New("unsupported setup")

// parseConfig reads key=value lines with shell-style values under optional
// [section] headers, and reports every problem at once.
func parseConfig(r io.Reader) (config, error) {
	vals := map[string]string{}
	section := ""
	sc := bufio.NewScanner(r)
	for n := 1; sc.Scan(); n++ {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			section = line[1 : len(line)-1]
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
	only("tax.form", "ryczalt")
	only("tax.payment", "monthly")
	only("tax.vat_status", "exempt")
	required("tax.business_start")
	c.businessStart = month("tax.business_start")
	c.revenueBasis = oneOf("tax.revenue_basis", "period_end", "service")
	if v := vals["tax.method_confirmed_on"]; v != "" {
		if _, err := time.Parse(time.DateOnly, v); err != nil {
			bad("tax.method_confirmed_on", fmt.Sprintf("%q is not a date like 2026-01-31", v))
		}
		c.methodConfirmedOn = v
	}
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
