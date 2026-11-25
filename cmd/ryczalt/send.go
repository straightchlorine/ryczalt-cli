package main

import (
	"bufio"
	"bytes"
	"cmp"
	"errors"
	"flag"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"time"
)

// syncExcludes keep Syncthing's own files out of the off-site copy.
var syncExcludes = []string{".stfolder", ".stignore", ".stversions/*", "*/.stversions/*", "*.sync-conflict-*"}

// syncArgs is the aws command line that pushes the archive to R2.
func syncArgs(c config, del, dry bool) []string {
	a := []string{"s3", "sync", c.archive, "s3://" + c.r2Bucket + "/"}
	if c.r2Profile != "" {
		a = append(a, "--profile", c.r2Profile)
	}
	if c.r2Endpoint != "" {
		a = append(a, "--endpoint-url", c.r2Endpoint)
	}
	for _, x := range syncExcludes {
		a = append(a, "--exclude", x)
	}
	if del {
		a = append(a, "--delete")
	}
	if dry {
		a = append(a, "--dryrun")
	}
	return a
}

// pushToR2 copies the archive up; it never pulls, so the archive stays the
// original and R2 the copy.
func pushToR2(c config, del, dry bool) error {
	cmd := exec.Command("aws", syncArgs(c, del, dry)...)
	cmd.Stdout, cmd.Stderr = os.Stdout, os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("aws s3 sync: %w", err)
	}
	return nil
}

func cmdSync(args []string) error {
	flags := flag.NewFlagSet("sync", flag.ContinueOnError)
	cfgFlag := flags.String("c", "", "config file")
	del := flags.Bool("delete", false, "also delete from R2 what is gone from the archive")
	dry := flags.Bool("dry-run", false, "show what would be copied")
	if err := parseFlags(flags, args); err != nil {
		return exitError{2, err}
	}
	if flags.NArg() > 0 {
		return exitError{2, errors.New("usage: ryczalt sync [-c CONFIG] [--delete] [--dry-run]")}
	}
	c, err := loadConfig(*cfgFlag)
	if err != nil {
		return err
	}
	if c.r2Bucket == "" {
		return exitError{2, errors.New("no [r2] bucket in the config; sync has nowhere to go")}
	}
	return pushToR2(c, *del, *dry)
}

var unsafeName = regexp.MustCompile(`[^A-Za-z0-9._-]`)

// ksefEnv reads env= from the ksef-cli config ksef will use.
func ksefEnv(c config) string {
	path := cmp.Or(c.ksefConfig, ksefConfigPath())
	b, err := os.ReadFile(path)
	if err != nil {
		return "test" // ksef-cli's default
	}
	sc := bufio.NewScanner(bytes.NewReader(b))
	for sc.Scan() {
		if k, v, ok := strings.Cut(strings.TrimSpace(sc.Text()), "="); ok && strings.TrimSpace(k) == "env" {
			return cmp.Or(parseValue(v), "test")
		}
	}
	return "test"
}

// cmdSend sends a month's invoice to KSeF through ksef-cli: ksef convert
// --send on the invoice.json, saving the FA(3) as ksef-NUMBER.xml and its
// receipt (UPO) beside it. ksef asks before it sends.
func cmdSend(args []string) error {
	flags := flag.NewFlagSet("send", flag.ContinueOnError)
	cfgFlag := flags.String("c", "", "config file")
	flags.Usage = func() {
		fmt.Fprintln(os.Stderr, "usage: ryczalt send [-c CONFIG] [MONTH]\n"+
			"MONTH is the month of work; by default the one invoice not yet in KSeF.")
	}
	if err := parseFlags(flags, args); err != nil {
		return exitError{2, err}
	}
	if flags.NArg() > 1 {
		flags.Usage()
		return exitError{2, errors.New("too many arguments")}
	}
	c, err := loadConfig(*cfgFlag)
	if err != nil {
		return err
	}
	invs, _, err := readArchive(c.archive, c.nip)
	if err != nil {
		return err
	}

	// drafts from the KSeF start on that have no KSeF number yet
	var todo []invoice
	for _, inv := range invs {
		if inv.ksefNumber == "" && filepath.Base(inv.source) == "invoice.json" &&
			!inv.issued.Before(time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC)) {
			todo = append(todo, inv)
		}
	}
	if flags.NArg() == 1 {
		month, err := time.Parse("2006-01", flags.Arg(0))
		if err != nil {
			return exitError{2, fmt.Errorf("month %q is not like 2026-11", flags.Arg(0))}
		}
		todo = slices.DeleteFunc(todo, func(inv invoice) bool {
			return inv.serviceEnd.Year() != month.Year() || inv.serviceEnd.Month() != month.Month()
		})
	}
	switch {
	case len(todo) == 0:
		return errors.New("no invoice waiting to be sent (every invoice.json has a KSeF number)")
	case len(todo) > 1:
		var names []string
		for _, inv := range todo {
			names = append(names, inv.number+" ("+inv.serviceEnd.Format("2006-01")+")")
		}
		return exitError{2, fmt.Errorf("several invoices wait to be sent: %s; name the month of work",
			strings.Join(names, ", "))}
	}
	inv := todo[0]
	src := filepath.Join(c.archive, inv.source)
	out := filepath.Join(filepath.Dir(src), "ksef-"+unsafeName.ReplaceAllString(inv.number, "-")+".xml")
	upo := strings.TrimSuffix(out, ".xml") + ".upo.xml"
	if _, err := os.Stat(upo); err == nil {
		return fmt.Errorf("%s exists: %s looks sent already", upo, inv.number)
	}

	w := newReport(os.Stderr)
	today := todayWarsaw()
	if env := ksefEnv(c); env != "prod" {
		w.printf("%s ksef-cli is set to env=%s: this goes to the KSeF %s environment, not to the real one\n",
			tag(w, yellow, "WARN"), env, env)
	}
	if !inv.issued.Equal(today) {
		w.printf("%s issued %s, sent today: KSeF will record it as Offline", tag(w, yellow, "WARN"),
			inv.issued.Format(time.DateOnly))
		if next := shiftDeadline("PL", inv.issued.AddDate(0, 0, 1)); today.After(next) {
			w.printf(", and later than the next working day (%s); consider ryczalt new --date today", next.Format(time.DateOnly))
		}
		w.println()
	}
	if err := w.flush(); err != nil {
		return err
	}

	ksefArgs := []string{"convert", "--send", "--sale-date", inv.serviceEnd.Format(time.DateOnly), src, out}
	cmd := exec.Command(cmp.Or(c.ksefBin, "ksef"), ksefArgs...)
	cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
	if c.ksefConfig != "" {
		cmd.Env = append(os.Environ(), "KSEF_CONFIG="+c.ksefConfig)
	}
	if err := cmd.Run(); err != nil {
		var notFound *exec.Error
		if errors.As(err, &notFound) || errors.Is(err, fs.ErrNotExist) {
			return fmt.Errorf("%s not found: install ksef-cli, or set bin= in [ksef]", cmd.Path)
		}
		return fmt.Errorf("ksef: %w", err)
	}
	if _, err := os.Stat(upo); err != nil {
		return nil // not sent (declined at the prompt, or still pending): nothing to back up
	}
	if c.r2Bucket != "" {
		return pushToR2(c, false, false)
	}
	return nil
}
