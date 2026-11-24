// ryczalt - ryczalt, ZUS and payment deadlines for Polish sole traders.
package main

import (
	"fmt"
	"os"
)

// Overridden at release via -ldflags "-X main.version=...".
var version = "dev"

const usage = `Usage: ryczalt rules [YEAR]
       ryczalt version
       ryczalt -h

Subcommands:
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
	case "rules":
		err = cmdRules(os.Args[2:])
	case "version":
		fmt.Println(version)
	default:
		fmt.Fprintf(os.Stderr, "%s unknown subcommand %q - run ryczalt -h for the list\n",
			paint(os.Stderr, red, "Error:"), cmd)
		os.Exit(2)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, paint(os.Stderr, red, "Error:"), err)
		os.Exit(1)
	}
}
