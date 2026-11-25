package main

import (
	"os"
	"runtime"
	"strings"
)

const (
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
