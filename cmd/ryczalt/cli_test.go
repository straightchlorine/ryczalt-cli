package main

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// run calls a command with stdout and stderr captured.
func run(t *testing.T, cmd func([]string) error, args ...string) (string, error) {
	t.Helper()
	f, err := os.CreateTemp(t.TempDir(), "out")
	if err != nil {
		t.Fatal(err)
	}

	stdout, stderr := os.Stdout, os.Stderr
	os.Stdout, os.Stderr = f, f
	err = cmd(args)
	os.Stdout, os.Stderr = stdout, stderr

	if err := f.Close(); err != nil {
		t.Fatal(err)
	}

	b, _ := os.ReadFile(f.Name())
	return string(b), err
}

// The commands end to end on a small archive: record, list, void, report.
func TestCommands(t *testing.T) {
	fakeNBPFlat(t)
	t.Setenv("XDG_CACHE_HOME", t.TempDir())

	files := map[string]string{}
	for _, m := range []string{"02", "03", "04"} {
		start := "2026-" + m + "-01"
		files["2026-"+m+"/ksef-INV-"+m+".xml"] = fa("INV-"+m, start, day(t, start).AddDate(0, 0, -1).Format("2006-01-02"))
		files["2026-"+m+"/ksef-INV-"+m+".upo.xml"] = upo([2]string{"INV-" + m, "K-" + m})
	}

	root := writeArchive(t, files)
	conf := strings.NewReplacer("/srv/invoices", root, "business_start=2025-06", "business_start=2026-01",
		"ulga_until=2025-12", "ulga_until=").Replace(validConfig)
	cfg := filepath.Join(t.TempDir(), "ryczalt.conf")
	mustWrite(t, cfg, []byte(conf))

	// January and February paid on their deadlines, March's ZUS late
	out, err := run(t, cmdPaid, "-c", cfg, "--on-due", "2026-01..2026-02")
	if err != nil || strings.Count(out, "[  OK  ]") != 6 {
		t.Fatalf("paid range: %v\n%s", err, out)
	}

	if _, err := run(t, cmdPaid, "-c", cfg, "2026-03", "zus", "--on", "2026-04-22"); err != nil {
		t.Fatal(err)
	}
	if _, err := run(t, cmdPaid, "-c", cfg, "2026-03", "zus"); err == nil || !strings.Contains(err.Error(), "nothing new to record") {
		t.Errorf("duplicate: %v", err)
	}

	if _, err := run(t, cmdPaid, "-c", cfg, "2026-03", "zus", "100"); err == nil {
		t.Error("an AMOUNT for zus as a whole must be refused")
	}
	if _, err := run(t, cmdPaid, "-c", cfg, "2026-02", "health", "10.50", "--on", "2026-03-25"); err != nil {
		t.Errorf("a further payment with an amount: %v", err)
	}

	// ZUS first, ryczalt later: a plain "paid MONTH" records only what is missing
	out, err = run(t, cmdPaid, "-c", cfg, "2026-03", "--on", "2026-04-24")
	if err != nil || !strings.Contains(out, "already recorded, left as is: 2026-03 social, 2026-03 health") ||
		strings.Count(out, "[  OK  ]") != 1 {
		t.Fatalf("paid after zus: %v\n%s", err, out)
	}

	for _, bad := range [][]string{{"2026-03", "ryczalt", "1e3"}, {"2026-03", "ryczalt", "0"},
		{"2026-03", "--on", "2099-01-01"}, {"2026-03", "--on", "2026-02-27"}} {
		var exit exitError
		if _, err := run(t, cmdPaid, append([]string{"-c", cfg}, bad...)...); !errors.As(err, &exit) || exit.code != 2 {
			t.Errorf("paid %v: want a usage error, got %v", bad, err)
		}
	}

	list, err := run(t, cmdPaid, "-c", cfg, "--list", "2026")
	if err != nil || strings.Count(list, "\n") != 11 { // header and 10 payments
		t.Fatalf("list: %v\n%s", err, list)
	}

	var extra string
	for _, l := range strings.Split(list, "\n") {
		if strings.Contains(l, "10.50") {
			extra = strings.Fields(l)[0]
		}
	}

	if out, err := run(t, cmdVoid, "-c", cfg, extra); err != nil || !strings.Contains(out, "voided "+extra) {
		t.Errorf("void: %v %s", err, out)
	}
	if _, err := run(t, cmdVoid, "-c", cfg, extra); err == nil {
		t.Error("voiding twice must fail")
	}

	status, err := run(t, cmdStatus, "-c", cfg, "--today", "2026-05-18")
	if !errors.Is(err, errFailed) {
		t.Errorf("April has no invoice after 15 May: want errFailed, got %v", err)
	}

	for _, want := range []string{
		"2026-01  INV-02", "[  OK  ]",
		"2026-03  INV-04     13 332.00     1 600  456.18  498.35  Mon 2026-04-20  [ WARN ] late, 2026-04-24",
		"2026-04  -                  -       n/a  456.18  498.35  Wed 2026-05-20  [ FAIL ] no invoice; due by 2026-05-15",
		"Next payment  Wed 2026-05-20, for 2026-04",
	} {
		if !strings.Contains(status, want) {
			t.Errorf("status lacks %q:\n%s", want, status)
		}
	}

	check, err := run(t, cmdCheck, "-c", cfg)
	if err != nil || !strings.Contains(check, "3 invoices | 9 payments") || !strings.Contains(check, "INV-04: KSeF recorded it as Offline") {
		t.Errorf("check: %v\n%s", err, check)
	}

	rules, err := run(t, cmdRules, "2026")
	if err != nil || !strings.Contains(rules, "social, preferential                     456.18  420.86 without chorobowa") {
		t.Errorf("rules: %v\n%s", err, rules)
	}

	var usage exitError
	if _, err := run(t, cmdRules, "26"); !errors.As(err, &usage) || usage.code != 2 {
		t.Errorf("rules 26: want exit 2, got %v", err)
	}

	missing := filepath.Join(t.TempDir(), "missing.conf")
	mustWrite(t, missing, []byte(strings.Replace(conf, root, "/nonexistent/archive", 1)))
	if _, err := run(t, cmdStatus, "-c", missing); !errors.As(err, &usage) || usage.code != 2 {
		t.Errorf("missing archive: want exit 2, got %v", err)
	}

	// a config that names another tax form exits 3
	bad := filepath.Join(t.TempDir(), "bad.conf")
	mustWrite(t, bad, []byte(strings.Replace(conf, "form=ryczalt", "form=liniowy", 1)))
	var exit exitError
	if _, err := run(t, cmdStatus, "-c", bad); !errors.As(err, &exit) || exit.code != 3 {
		t.Errorf("unsupported setup: %v", err)
	}
}

func mustWrite(t *testing.T, path string, data []byte) {
	t.Helper()
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
}
