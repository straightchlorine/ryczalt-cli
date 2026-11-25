package main

import (
	"os"
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
