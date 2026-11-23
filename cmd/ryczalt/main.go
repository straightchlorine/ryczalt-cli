// ryczalt - ryczalt, ZUS and payment deadlines for Polish sole traders.
package main

import (
	"fmt"
	"os"
)

// Overridden at release via -ldflags "-X main.version=...".
var version = "dev"

const usage = `Usage: ryczalt version
       ryczalt -h
`

func main() {
	if len(os.Args) < 2 || os.Args[1] == "-h" || os.Args[1] == "--help" {
		fmt.Print(usage)
		return
	}

	switch cmd := os.Args[1]; cmd {
	case "version":
		fmt.Println(version)
	default:
		fmt.Fprintf(os.Stderr, "Error: unknown subcommand %q - run ryczalt -h for the list\n", cmd)
		os.Exit(2)
	}
}
