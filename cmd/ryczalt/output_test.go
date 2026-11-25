package main

import (
	"flag"
	"os"
	"strings"
	"testing"
)

func TestTagWidth(t *testing.T) {
	// not a terminal, so no colour codes
	f, err := os.CreateTemp(t.TempDir(), "out")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	for word, want := range map[string]string{"OK": "[  OK  ]", "WARN": "[ WARN ]", "FAIL": "[ FAIL ]", "SKIPPED": "[SKIPPED]"} {
		if got := tag(f, yellow, word); got != want {
			t.Errorf("tag(%q) = %q, want %q", word, got, want)
		}
	}
}

func TestParseFlags(t *testing.T) {
	flags := flag.NewFlagSet("x", flag.ContinueOnError)
	on := flags.String("on", "", "")
	due := flags.Bool("on-due", false, "")
	if err := parseFlags(flags, []string{"2026-10", "--on", "2026-11-18", "zus", "--on-due"}); err != nil {
		t.Fatal(err)
	}
	if *on != "2026-11-18" || !*due || strings.Join(flags.Args(), " ") != "2026-10 zus" {
		t.Errorf("on=%q due=%v args=%q", *on, *due, flags.Args())
	}
	flags = flag.NewFlagSet("x", flag.ContinueOnError)
	flags.Bool("v", false, "")
	if err := parseFlags(flags, []string{"a", "--", "-v"}); err != nil || strings.Join(flags.Args(), " ") != "a -v" {
		t.Errorf("after --: %q %v", flags.Args(), err)
	}
}
