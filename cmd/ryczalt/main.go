// ryczalt - ryczalt, ZUS and payment deadlines for Polish sole traders.
package main

import (
	"errors"
	"fmt"
	"os"
)

// Overridden at release via -ldflags "-X main.version=...".
var version = "dev"

const usage = `Usage: ryczalt init [-c CONFIG] [ARCHIVE]
       ryczalt status [-c CONFIG] [--year YEAR] [--today DATE]
       ryczalt rules [YEAR]
       ryczalt version
       ryczalt -h

Subcommands:
  init        Write a starter config to $RYCZALT_CONFIG, else
              $XDG_CONFIG_HOME/ryczalt.conf (default ~/.config/ryczalt.conf),
              never overwriting one. ARCHIVE is the folder of YYYY-MM
              invoice folders (default ~/invoices). The NIP is taken from
              ksef-cli's ksef.conf when there is one.
  status      Month by month for the year: revenue from the invoices in the
              archive (in PLN at the NBP rate the PIT act requires), ryczalt,
              social and health contributions, the payment deadline and what
              still needs doing. Exits 1 when something is overdue.
  rules       The figures a year's contributions are computed from, with
              their source and whether they are official, a forecast or not
              published yet, and the monthly health and social contributions
              that follow from them. YEAR defaults to this year.
  version     Print version.
`

func main() {
	if len(os.Args) < 2 || os.Args[1] == "-h" || os.Args[1] == "--help" {
		fmt.Print(usage)
		return
	}

	var err error
	switch cmd := os.Args[1]; cmd {
	case "init":
		err = cmdInit(os.Args[2:])
	case "status":
		err = cmdStatus(os.Args[2:])
	case "rules":
		err = cmdRules(os.Args[2:])
	case "version", "--version", "-v":
		fmt.Println(version)
	default:
		fmt.Fprintf(os.Stderr, "%s unknown subcommand %q - run ryczalt -h for the list\n",
			paint(os.Stderr, red, "Error:"), cmd)
		os.Exit(2)
	}
	var exit exitError
	switch {
	case err == nil:
	case errors.Is(err, errFailed):
		os.Exit(1)
	case errors.As(err, &exit):
		fmt.Fprintln(os.Stderr, paint(os.Stderr, red, "Error:"), err)
		os.Exit(exit.code)
	default:
		fmt.Fprintln(os.Stderr, paint(os.Stderr, red, "Error:"), err)
		os.Exit(1)
	}
}
