package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// fakeTool writes an executable shell script and returns its path.
func fakeTool(t *testing.T, name, script string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(p, []byte("#!/bin/sh\n"+script), 0o755); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestAdd(t *testing.T) {
	root := writeArchive(t, map[string]string{
		"2026-10/ksef-INV-10.xml": fa("INV-10", "2026-10-01", "2026-09-30"),
		"2026-11/ksef-INV-11.xml": fa("INV-11", "2026-11-01", "2026-10-31"),
	})

	cfg := filepath.Join(t.TempDir(), "ryczalt.conf")
	mustWrite(t, cfg, []byte(strings.Replace(validConfig, "/srv/invoices", root, 1)))

	downloads := t.TempDir()
	pdf := filepath.Join(downloads, "INV-11.pdf")
	mustWrite(t, pdf, []byte("%PDF-1.7 invoice"))

	if _, err := run(t, cmdAdd, "-c", cfg, pdf); err != nil {
		t.Fatal(err)
	}

	if b, err := os.ReadFile(filepath.Join(root, "2026-11", "INV-11.pdf")); err != nil || string(b) != "%PDF-1.7 invoice" {
		t.Errorf("not copied into the newest invoice's month: %v", err)
	}
	if _, err := os.Stat(pdf); err != nil {
		t.Error("a copy must leave the original")
	}

	if _, err := run(t, cmdAdd, "-c", cfg, pdf); err == nil || !strings.Contains(err.Error(), "already exists") {
		t.Errorf("second add: %v", err)
	}

	// --move (-m) into a month given by hand, a folder that does not exist yet
	if _, err := run(t, cmdAdd, "-c", cfg, "-m", "--month", "2026-12", pdf); err != nil {
		t.Fatal(err)
	}

	if _, err := os.Stat(filepath.Join(root, "2026-12", "INV-11.pdf")); err != nil {
		t.Error(err)
	}
	if _, err := os.Stat(pdf); !os.IsNotExist(err) {
		t.Error("--move must remove the original")
	}

	if _, err := run(t, cmdAdd, "-c", cfg, "--month", "12-2026", filepath.Join(root, "2026-12", "INV-11.pdf")); err == nil {
		t.Error("want an error for a month written backwards")
	}
}

func TestSendThenSync(t *testing.T) {
	var nbpCalls []string
	fakeNBP(t, &nbpCalls) // its last table is 212/A of 2026-10-30, 4.3955
	t.Setenv("XDG_CACHE_HOME", t.TempDir())

	log := filepath.Join(t.TempDir(), "calls.log")
	t.Setenv("CALLS", log)

	// ksef: record the call, write the FA(3) and a UPO the way ksef-cli does
	ksef := fakeTool(t, "ksef", `echo "ksef $* KSEF_CONFIG=$KSEF_CONFIG" >> "$CALLS"
for a; do out=$a; done
cat "$FA" > "$out"
echo '<Potwierdzenie><Dokument><NumerKSeFDokumentu>K-11</NumerKSeFDokumentu><NumerFaktury>INV-11</NumerFaktury><TrybWysylki>Online</TrybWysylki></Dokument></Potwierdzenie>' > "${out%.xml}.upo.xml"
`)
	aws := fakeTool(t, "aws", `echo "aws $*" >> "$CALLS"`)
	t.Setenv("PATH", filepath.Dir(aws)+":"+os.Getenv("PATH"))

	faFile := filepath.Join(t.TempDir(), "fa.xml")
	mustWrite(t, faFile, []byte(fa("INV-11", "2026-11-01", "2026-10-31")))
	t.Setenv("FA", faFile)

	root := writeArchive(t, map[string]string{
		"2026-11/invoice.json": strings.NewReplacer("DEV-2026-10-001", "INV-11", "2026-10-01T", "2026-11-01T").Replace(invoifyTemplate),
	})

	ksefConf := filepath.Join(t.TempDir(), "ksef.conf")
	mustWrite(t, ksefConf, []byte("env=prod\n"))
	cfg := filepath.Join(t.TempDir(), "ryczalt.conf")
	mustWrite(t, cfg, []byte(strings.Replace(validConfig, "/srv/invoices", root, 1)+
		"[ksef]\nbin="+ksef+"\nconfig="+ksefConf+"\n[r2]\nbucket=invoices\nprofile=r2\n"))

	out, err := run(t, cmdSend, "-c", cfg)
	if err != nil {
		t.Fatal(err)
	}

	if want := "[ INFO ] rate 4.3955 PLN per EUR, NBP table 212/A/NBP/2026 of 2026-10-30, as in the revenue register\n"; !strings.Contains(out, want) {
		t.Errorf("report:\n%s\nwant the line %q", out, want)
	}

	b, _ := os.ReadFile(log)
	calls := strings.Split(strings.TrimSpace(string(b)), "\n")

	dir := filepath.Join(root, "2026-11")
	wantKsef := "ksef convert --send --sale-date 2026-10-31 --rate 4.3955 " + filepath.Join(dir, "invoice.json") + " " +
		filepath.Join(dir, "ksef-INV-11.xml") + " KSEF_CONFIG=" + ksefConf
	if len(calls) != 2 || calls[0] != wantKsef || !strings.HasPrefix(calls[1], "aws s3 sync "+root+" s3://invoices/ --profile r2 --exclude .stfolder") {
		t.Errorf("calls:\n%s\nwant first\n%s", b, wantKsef)
	}

	if _, err := os.Stat(filepath.Join(dir, "ksef-INV-11.upo.xml")); err != nil {
		t.Error(err)
	}

	// sent now: nothing more to send, and the archive reads the UPO
	if _, err := run(t, cmdSend, "-c", cfg); err == nil || !strings.Contains(err.Error(), "no invoice waiting") {
		t.Errorf("second send: %v", err)
	}

	invs, _, err := readArchive(root, testNIP)
	if err != nil || len(invs) != 1 || invs[0].ksefNumber != "K-11" {
		t.Errorf("after send: %+v %v", invs, err)
	}

	// sync on its own, with --delete and --dry-run
	if _, err := run(t, cmdSync, "-c", cfg, "--delete", "--dry-run"); err != nil {
		t.Fatal(err)
	}

	b, _ = os.ReadFile(log)
	if last := strings.Split(strings.TrimSpace(string(b)), "\n"); !strings.HasSuffix(last[len(last)-1], "--delete --dryrun") {
		t.Errorf("sync flags: %s", last[len(last)-1])
	}
}

// A PLN invoice gets no --rate: ksef-cli rejects one.
func TestSendPLN(t *testing.T) {
	log := filepath.Join(t.TempDir(), "calls.log")
	t.Setenv("CALLS", log)

	ksef := fakeTool(t, "ksef", `echo "ksef $*" >> "$CALLS"`)
	root := writeArchive(t, map[string]string{
		"2026-11/invoice.json": strings.NewReplacer(
			"DEV-2026-10-001", "INV-11", "2026-10-01T", "2026-11-01T", `"currency":"EUR"`, `"currency":"PLN"`,
		).Replace(invoifyTemplate),
	})

	cfg := filepath.Join(t.TempDir(), "ryczalt.conf")
	mustWrite(t, cfg, []byte(strings.Replace(validConfig, "/srv/invoices", root, 1)+"[ksef]\nbin="+ksef+"\n"))

	if _, err := run(t, cmdSend, "-c", cfg); err != nil {
		t.Fatal(err)
	}

	b, _ := os.ReadFile(log)
	if got := string(b); strings.Contains(got, "--rate") || !strings.HasPrefix(got, "ksef convert --send --sale-date 2026-10-31 ") {
		t.Errorf("calls: %s", got)
	}
}

// Declining at ksef-cli's prompt leaves the prepared XML and no UPO; send
// offers the invoice again from its draft.
func TestSendAgainAfterDeclined(t *testing.T) {
	log := filepath.Join(t.TempDir(), "calls.log")
	t.Setenv("CALLS", log)

	faFile := filepath.Join(t.TempDir(), "fa.xml")
	mustWrite(t, faFile, []byte(fa("INV-11", "2026-11-01", "2026-10-31")))
	t.Setenv("FA", faFile)

	ksef := fakeTool(t, "ksef", `echo "$*" >> "$CALLS"
for a; do out=$a; done
cat "$FA" > "$out"
`)
	root := writeArchive(t, map[string]string{
		"2026-11/invoice.json": strings.NewReplacer("DEV-2026-10-001", "INV-11", "2026-10-01T", "2026-11-01T", `"currency":"EUR"`, `"currency":"PLN"`).Replace(invoifyTemplate),
	})

	cfg := filepath.Join(t.TempDir(), "ryczalt.conf")
	mustWrite(t, cfg, []byte(strings.Replace(validConfig, "/srv/invoices", root, 1)+"[ksef]\nbin="+ksef+"\n"))

	if _, err := run(t, cmdSend, "-c", cfg); err != nil {
		t.Fatal(err)
	}

	// the draft is corrected before the second try: issued a month later
	draft := filepath.Join(root, "2026-11", "invoice.json")
	b, _ := os.ReadFile(draft)
	mustWrite(t, draft, []byte(strings.Replace(string(b), "2026-11-01T", "2026-12-01T", 1)))

	if _, err := run(t, cmdSend, "-c", cfg); err != nil {
		t.Fatal(err)
	}

	b, _ = os.ReadFile(log)
	calls := strings.Split(strings.TrimSpace(string(b)), "\n")
	if len(calls) != 2 || !strings.Contains(calls[1], " "+draft+" ") {
		t.Fatalf("calls:\n%s", b)
	}

	// the second call follows the draft as it is now, not the XML left behind
	if !strings.Contains(calls[0], "--sale-date 2026-10-31") || !strings.Contains(calls[1], "--sale-date 2026-11-30") {
		t.Errorf("sale dates:\n%s", b)
	}
}

func TestSendRefusals(t *testing.T) {
	root := writeArchive(t, map[string]string{
		"2026-10/invoice.json": invoifyTemplate,
		"2026-11/invoice.json": strings.NewReplacer("DEV-2026-10-001", "INV-11", "2026-10-01T", "2026-11-01T").Replace(invoifyTemplate),
	})

	cfg := filepath.Join(t.TempDir(), "ryczalt.conf")
	mustWrite(t, cfg, []byte(strings.Replace(validConfig, "/srv/invoices", root, 1)+"[ksef]\nbin=/nonexistent/ksef\n"))

	if _, err := run(t, cmdSend, "-c", cfg); err == nil || !strings.Contains(err.Error(), "several invoices wait") {
		t.Errorf("two drafts: %v", err)
	}
	if _, err := run(t, cmdSend, "-c", cfg, "2026-10"); err == nil || !strings.Contains(err.Error(), "install ksef-cli") {
		t.Errorf("missing ksef: %v", err)
	}
	if _, err := run(t, cmdSend, "-c", cfg, "2026-08"); err == nil || err.Error() != "no invoice for work in 2026-08 is waiting to be sent" {
		t.Errorf("month without a draft: %v", err)
	}
	if _, err := run(t, cmdSync, "-c", cfg); err == nil || !strings.Contains(err.Error(), "no [r2] bucket") {
		t.Errorf("sync without r2: %v", err)
	}
}
