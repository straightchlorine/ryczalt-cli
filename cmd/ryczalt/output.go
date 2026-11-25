package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"runtime"
	"strings"
	"unicode/utf8"
)

const (
	bold, dim                = "1", "2"
	red, green, yellow, cyan = "31", "32", "33", "36"
)

// report collects output and writes it in one go, so a failed write (a
// closed pipe, a full disk) is returned once instead of lost line by line.
type report struct {
	out *os.File // where it goes; also decides colours
	b   strings.Builder
}

func newReport(f *os.File) *report { return &report{out: f} }

func (r *report) Write(p []byte) (int, error)    { return r.b.Write(p) }
func (r *report) printf(format string, a ...any) { fmt.Fprintf(&r.b, format, a...) }
func (r *report) println(a ...any)               { fmt.Fprintln(&r.b, a...) }

func (r *report) flush() error {
	_, err := r.out.WriteString(r.b.String())
	r.b.Reset()
	return err
}

func isTerminal(w io.Writer) bool {
	if r, ok := w.(*report); ok {
		w = r.out
	}
	f, ok := w.(*os.File)
	if !ok {
		return false
	}
	st, err := f.Stat()
	return err == nil && st.Mode()&os.ModeCharDevice != 0
}

// colours says whether w is a terminal that takes ANSI codes.
// NO_COLOR and TERM=dumb switch them off.
func colours(w io.Writer) bool {
	return isTerminal(w) && os.Getenv("NO_COLOR") == "" && os.Getenv("TERM") != "dumb" &&
		// the old Windows console prints the codes instead of colours
		(runtime.GOOS != "windows" || os.Getenv("WT_SESSION") != "")
}

func paint(w io.Writer, colour, s string) string {
	if !colours(w) {
		return s
	}
	return "\x1b[" + colour + "m" + s + "\x1b[0m"
}

// tag is a status tag of fixed width: [  OK  ], [ WARN ], [ FAIL ], [ INFO ].
func tag(w io.Writer, colour, word string) string {
	pad := max(0, 6-len(word))
	return paint(w, colour, "["+strings.Repeat(" ", pad/2)+word+strings.Repeat(" ", pad-pad/2)+"]")
}

// printTable prints rows in aligned columns: the first row is the header,
// right names the columns of numbers, and colour paints a finished cell
// (padding first, so codes never upset the alignment). Copied from
// ksef-cli (cmd/ksef/output.go), with several number columns.
func printTable(w *report, rows [][]string, right []bool, colour func(row, col int) string) {
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
		w.println(strings.TrimRight(strings.Join(cells, "  "), " "))
	}
}

// parseFlags is flags.Parse that also takes flags after the arguments,
// as in "ryczalt paid 2026-10 --on 2026-11-18"; "--" ends the flags.
func parseFlags(flags *flag.FlagSet, args []string) error {
	// flag errors are printed once, by main; help goes to stderr
	flags.SetOutput(io.Discard)
	var positional []string
	for {
		if err := flags.Parse(args); err != nil {
			if errors.Is(err, flag.ErrHelp) {
				flags.SetOutput(os.Stderr)
				flags.Usage()
			}
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
