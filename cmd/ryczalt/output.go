package main

import (
	"flag"
	"fmt"
	"os"
	"runtime"
	"strings"
	"unicode/utf8"
)

const (
	bold, dim                = "1", "2"
	red, green, yellow, cyan = "31", "32", "33", "36"
)

func isTerminal(w *os.File) bool {
	st, err := w.Stat()
	return err == nil && st.Mode()&os.ModeCharDevice != 0
}

// colours says whether w is a terminal that takes ANSI codes.
// NO_COLOR and TERM=dumb switch them off.
func colours(w *os.File) bool {
	return isTerminal(w) && os.Getenv("NO_COLOR") == "" && os.Getenv("TERM") != "dumb" &&
		// the old Windows console prints the codes instead of colours
		(runtime.GOOS != "windows" || os.Getenv("WT_SESSION") != "")
}

func paint(w *os.File, colour, s string) string {
	if !colours(w) {
		return s
	}
	return "\x1b[" + colour + "m" + s + "\x1b[0m"
}

// tag is a status tag of fixed width: [  OK  ], [ WARN ], [ FAIL ], [ INFO ].
func tag(w *os.File, colour, word string) string {
	pad := max(0, 6-len(word))
	return paint(w, colour, "["+strings.Repeat(" ", pad/2)+word+strings.Repeat(" ", pad-pad/2)+"]")
}

// printTable prints rows in aligned columns: the first row is the header,
// right names the columns of numbers, and colour paints a finished cell
// (padding first, so codes never upset the alignment). Copied from
// ksef-cli (cmd/ksef/output.go), with several number columns.
func printTable(w *os.File, rows [][]string, right []bool, colour func(row, col int) string) {
	widths := make([]int, len(rows[0]))
	for _, row := range rows {
		for i, cell := range row {
			widths[i] = max(widths[i], utf8.RuneCountInString(cell))
		}
	}
	for r, row := range rows {
		cells := make([]string, len(row))
		for i, cell := range row {
			pad := strings.Repeat(" ", widths[i]-utf8.RuneCountInString(cell))
			if right[i] {
				cell = pad + cell
			} else if i < len(row)-1 {
				cell += pad
			}
			if c := colour(r, i); c != "" {
				cell = paint(w, c, cell)
			}
			cells[i] = cell
		}
		fmt.Fprintln(w, strings.TrimRight(strings.Join(cells, "  "), " "))
	}
}

// parseFlags is flags.Parse that also takes flags after the arguments,
// as in "ryczalt paid 2026-10 --on 2026-11-18"; "--" ends the flags.
func parseFlags(flags *flag.FlagSet, args []string) error {
	var positional []string
	for {
		if err := flags.Parse(args); err != nil {
			return err
		}
		args = flags.Args()
		if len(args) == 0 {
			break
		}
		if args[0] == "--" {
			positional = append(positional, args[1:]...)
			break
		}
		positional, args = append(positional, args[0]), args[1:]
	}
	return flags.Parse(append([]string{"--"}, positional...))
}
