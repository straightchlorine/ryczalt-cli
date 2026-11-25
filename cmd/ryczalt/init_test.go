package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// ryczalt.conf.example is what init writes, with placeholders.
func TestConfigExampleIsTemplate(t *testing.T) {
	b, err := os.ReadFile("../../ryczalt.conf.example")
	if err != nil {
		t.Fatal(err)
	}
	if want := fmt.Sprintf(confTemplate, "~/invoices", "1234563218"); string(b) != want {
		t.Errorf("ryczalt.conf.example differs from confTemplate; regenerate it")
	}
}

func TestInit(t *testing.T) {
	dir := t.TempDir()
	ksef := filepath.Join(dir, "ksef.conf")
	os.WriteFile(ksef, []byte("env=prod\nnip = \"5260250274\"  # mine\nksef_token=\n"), 0o600)
	t.Setenv("KSEF_CONFIG", ksef)
	path := filepath.Join(dir, "sub", "ryczalt.conf")

	if err := cmdInit([]string{"-c", path, "/srv/invoices"}); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(b), "archive=/srv/invoices\n") || !strings.Contains(string(b), "nip=5260250274\n") {
		t.Errorf("config:\n%s", b)
	}
	if st, _ := os.Stat(path); st.Mode().Perm() != 0o600 {
		t.Errorf("mode %v, want 0600", st.Mode().Perm())
	}
	// what is left blank is exactly what the user has to fill in
	_, err = parseConfig(strings.NewReader(string(b)))
	want := "tax.business_start: required\ntax.revenue_basis: required\nzus.chorobowa: required"
	if err == nil || err.Error() != want {
		t.Errorf("fresh config: got %v, want\n%s", err, want)
	}

	if err := cmdInit([]string{"-c", path}); err == nil || !strings.Contains(err.Error(), "already exists") {
		t.Errorf("second init: %v", err)
	}
	if err := cmdInit([]string{"-c", filepath.Join(dir, "x.conf"), "invoices"}); err == nil {
		t.Error("want an error for a relative archive")
	}

	// no ksef.conf: nip stays blank
	t.Setenv("KSEF_CONFIG", filepath.Join(dir, "none.conf"))
	other := filepath.Join(dir, "other.conf")
	if err := cmdInit([]string{"-c", other}); err != nil {
		t.Fatal(err)
	}
	if b, _ := os.ReadFile(other); !strings.Contains(string(b), "nip=\n") {
		t.Error("want a blank nip without ksef.conf")
	}
}
