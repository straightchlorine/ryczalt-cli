package main

import (
	"bufio"
	"errors"
	"flag"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// confTemplate is what init writes; ryczalt.conf.example is the same text
// (a test keeps them equal). %[1]s is the archive, %[2]s the NIP.
const confTemplate = `# ryczalt configuration. Values follow shell rules: quotes are optional,
# " #" starts a comment. Unknown keys are errors, so a typo is reported
# instead of ignored.

# Folder holding one YYYY-MM folder per month of invoices.
archive=%[1]s

[tax]   # podatek
# Your NIP. Invoices in the archive issued by anyone else (e.g. purchase
# invoices pulled from KSeF) are not revenue and are skipped.
nip=%[2]s

# The setup this tool handles. Anything else is refused.
form=ryczalt
payment=monthly
vat_status=exempt

# Ryczałt rate in percent for your services (art. 12 ustawy o
# zryczałtowanym podatku): 12 for most software work, 8.5 for other IT
# services. Check it against your PKWiU code.
rate=12

# First month of the business (YYYY-MM).
business_start=

# When revenue arises; ask your accountant, the tool does not decide:
#   period_end  last day of the settlement period agreed in the contract
#               (art. 14 ust. 1e ustawy o PIT), e.g. a monthly retainer
#   service     the day the service is done, unless the invoice or the
#               payment comes earlier (art. 14 ust. 1c)
revenue_basis=

[zus]
# Last month of ulga na start and of preferential contributions (YYYY-MM),
# as PUE ZUS shows them. Leave blank if you never had one.
ulga_until=
preferential_until=

# yes if you pay the voluntary sickness insurance (ubezpieczenie
# chorobowe), no if you pay only the obligatory contributions.
chorobowa=
`

// ksefConfigPath is where ksef-cli keeps its config: $KSEF_CONFIG, else
// $XDG_CONFIG_HOME/ksef.conf (default ~/.config/ksef.conf).
func ksefConfigPath() string {
	if p := os.Getenv("KSEF_CONFIG"); p != "" {
		return p
	}
	return xdgPath("XDG_CONFIG_HOME", ".config", "ksef.conf")
}

// nipFromKsef reads nip= from ksef-cli's config, if it has a valid one.
func nipFromKsef(path string) string {
	f, err := os.Open(path)
	if err != nil {
		return ""
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		k, v, ok := strings.Cut(strings.TrimSpace(sc.Text()), "=")
		if ok && strings.TrimSpace(k) == "nip" && validNIP(parseValue(v)) {
			return parseValue(v)
		}
	}
	return ""
}

// cmdInit writes a starter config and never overwrites an existing one.
// The NIP comes from ksef-cli's config when there is one.
func cmdInit(args []string) error {
	flags := flag.NewFlagSet("init", flag.ContinueOnError)
	cfgFlag := flags.String("c", "", "where to write the config")
	if err := parseFlags(flags, args); err != nil {
		return exitError{2, err}
	}
	archive := "~/invoices"
	switch flags.NArg() {
	case 0:
	case 1:
		archive = flags.Arg(0)
	default:
		return exitError{2, errors.New("usage: ryczalt init [-c CONFIG] [ARCHIVE]")}
	}
	if !filepath.IsAbs(expandHome(archive)) {
		return exitError{2, fmt.Errorf("archive %q must be an absolute path or start with ~/", archive)}
	}

	ksef := ksefConfigPath()
	nip := nipFromKsef(ksef)
	path := configPath(*cfgFlag)
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if errors.Is(err, fs.ErrExist) {
		return fmt.Errorf("%s already exists, leaving it alone", path)
	}
	if err != nil {
		return err
	}
	if _, err := fmt.Fprintf(f, confTemplate, archive, nip); err != nil {
		f.Close()
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}

	out := os.Stdout
	fmt.Fprintf(out, "%s wrote %s\n", tag(out, green, "OK"), path)
	todo := []string{"tax.business_start", "tax.revenue_basis", "zus.chorobowa",
		"zus.ulga_until and zus.preferential_until, if you had them"}
	if nip != "" {
		fmt.Fprintf(out, "%s nip taken from ksef-cli's %s\n", tag(out, cyan, "INFO"), ksef)
	} else {
		todo = append([]string{"tax.nip"}, todo...)
	}
	fmt.Fprintln(out, "  fill in, then run ryczalt status:")
	for _, t := range todo {
		fmt.Fprintln(out, "  -", t)
	}
	return nil
}
