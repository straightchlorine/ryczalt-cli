// ryczalt - ryczalt, ZUS and payment deadlines for Polish sole traders.
package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
)

// Overridden at release via -ldflags "-X main.version=...".
var version = "dev"

const usage = `Usage: ryczalt init [-c CONFIG] [ARCHIVE]
       ryczalt new [-c CONFIG] [--date DATE] [--number NUMBER] [MONTH]
       ryczalt add [-c CONFIG] [--month YYYY-MM] [--move|-m] FILE...
       ryczalt send [-c CONFIG] [MONTH]
       ryczalt sync [-c CONFIG] [--delete] [--dry-run]
       ryczalt status [-c CONFIG] [--year YEAR] [--today DATE]
       ryczalt paid [-c CONFIG] [--on DATE | --on-due] PERIOD [KIND [AMOUNT]]
       ryczalt paid [-c CONFIG] --list [YEAR]
       ryczalt void [-c CONFIG] ID...
       ryczalt year [-c CONFIG] [--today DATE] [YEAR]
       ryczalt ewid [-c CONFIG] [--csv | --jpk [--correction]] [YEAR]
       ryczalt forms [-c CONFIG] [--today DATE] [YEAR]
       ryczalt check [-c CONFIG]
       ryczalt rules [YEAR]
       ryczalt version
       ryczalt -h

Subcommands:
  init        Write a starter config to $RYCZALT_CONFIG, else
              $XDG_CONFIG_HOME/ryczalt.conf (default ~/.config/ryczalt.conf),
              never overwriting one. ARCHIVE is the folder of YYYY-MM
              invoice folders (default ~/invoices). The NIP is taken from
              ksef-cli's ksef.conf when there is one.
  new         Prepare the invoice for a month of work (default last month)
              as an Invoify invoice.json in the folder of the issue month,
              copied from the previous one: number, dates, work period, the NBP
              rate the PIT act asks for and the PLN amount filled in. Import it
              into Invoify, check it, export the PDF. Shows what the month
              will cost.
  add         Put documents (the invoice PDF from Invoify, a contract, a
              receipt) into a month folder, by default the newest invoice's.
              --move removes the originals once copied. Never overwrites.
  send        Send a month's invoice to KSeF with ksef-cli (default: the one
              invoice.json not yet sent), keeping the FA(3) and its UPO in
              the month folder; ksef-cli asks before sending. Then sync.
  sync        Push the archive to the [r2] bucket with the aws CLI. Never
              pulls; --delete also removes from R2 what the archive lost.
  status      Month by month for the year: revenue from the invoices in the
              archive (in PLN at the NBP rate the PIT act requires), ryczalt,
              social and health contributions, the payment deadline and what
              still needs doing. Exits 1 when something is overdue or an
              invoice is missing after its deadline.
  paid        Record a payment for a month (2026-10) or a range of months
              (2026-01..2026-10): KIND is zus, social, health, fpfs or
              ryczalt, by default zus and ryczalt at the amounts due. Paid
              today unless --on DATE, or --on-due for each month's deadline.
              AMOUNT records a different amount, or a further payment.
              --list shows a year's payments with their ids.
  void        Cancel recorded payments by id; nothing is deleted.
  year        The year settled: PIT-28 figures (revenue, the social and half
              the health contributions paid that year, ryczalt, advances,
              what is left to pay or comes back) and the annual health
              settlement.
  ewid        The year's revenue register (ewidencja przychodów): a table, or
              --csv with the register's 17 columns, or --jpk for the JPK_EWP(4)
              file sent with PIT-28 (needs [jpk] in the config).
  forms       What goes into PIT-28(28) and block XII of the ZUS DRA for April,
              field by field, from the year's figures.
  check       Everything worth knowing about the archive, beyond what status
              shows: drafts that differ from what was sent, invoices without
              a KSeF number, invoices issued late, session keys left in clear
              text. Exits 1 when any of it is a warning.
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
	case "new":
		err = cmdNew(os.Args[2:])
	case "add":
		err = cmdAdd(os.Args[2:])
	case "send":
		err = cmdSend(os.Args[2:])
	case "sync":
		err = cmdSync(os.Args[2:])
	case "status":
		err = cmdStatus(os.Args[2:])
	case "paid":
		err = cmdPaid(os.Args[2:])
	case "void":
		err = cmdVoid(os.Args[2:])
	case "year":
		err = cmdYear(os.Args[2:])
	case "ewid":
		err = cmdEwid(os.Args[2:])
	case "forms":
		err = cmdForms(os.Args[2:])
	case "check":
		err = cmdCheck(os.Args[2:])
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
	case err == nil, errors.Is(err, flag.ErrHelp):
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
