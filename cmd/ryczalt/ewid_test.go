package main

import (
	"encoding/csv"
	"errors"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func ewidArchive(t *testing.T) (cfg, root string) {
	t.Helper()
	fakeNBPFlat(t)
	oldClock := clock // December's rate is out by now
	clock = func() time.Time { return time.Date(2027, 2, 1, 12, 0, 0, 0, warsaw) }
	t.Cleanup(func() { clock = oldClock })
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	vatBuyer := strings.Replace(fa("INV-05", "2026-05-02", "2026-04-30"), "<Fa>",
		"<Podmiot2><DaneIdentyfikacyjne><KodUE>DE</KodUE><NrVatUE>123456789</NrVatUE><Nazwa>Client GmbH</Nazwa></DaneIdentyfikacyjne></Podmiot2><Fa>", 1)
	idBuyer := strings.Replace(fa("INV-04", "2026-04-01", "2026-03-31"), "<Fa>",
		"<Podmiot2><DaneIdentyfikacyjne><KodKraju>DE</KodKraju><NrID>HRB 12345</NrID><Nazwa>Client GmbH</Nazwa></DaneIdentyfikacyjne></Podmiot2><Fa>", 1)
	root = writeArchive(t, map[string]string{
		"2026-04/ksef-INV-04.xml":     idBuyer,
		"2026-04/ksef-INV-04.upo.xml": upo([2]string{"INV-04", "5260250274-20260401-ABCDEF123456-AB"}),
		"2026-05/ksef-INV-05.xml":     vatBuyer,
		"2026-03/invoice.json":        invoify("INV-03", "2026-03-02T00:00:00.000Z"),
		"2027-01/ksef-INV-01.xml":     fa("INV-12", "2027-01-04", "2026-12-31"), // December's work: 2026
	})
	cfg = filepath.Join(t.TempDir(), "ryczalt.conf")
	mustWrite(t, cfg, []byte(strings.Replace(validConfig, "/srv/invoices", root, 1)+
		"[jpk]\nfirst_name=Jan\nsurname=Testowy\nbirth_date=1990-01-01\ntax_office=1471\n"))
	return cfg, root
}

func TestEwidCSV(t *testing.T) {
	cfg, _ := ewidArchive(t)
	out, err := run(t, cmdEwid, "-c", cfg, "--csv", "2026")
	if err != nil {
		t.Fatal(err)
	}
	recs, err := csv.NewReader(strings.NewReader(out)).ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	if len(recs) != 5 || len(recs[0]) != 17 || recs[0][10] != "12%" || recs[0][15] != "Ogółem" {
		t.Fatalf("want a header and 4 rows of 17 columns:\n%s", out)
	}
	want := [][]string{
		{"1", "2026-03-02", "2026-02-28", "", "INV-03", "", "13332.00"},
		{"2", "2026-04-01", "2026-03-31", "5260250274-20260401-ABCDEF123456-AB", "INV-04", "DE HRB 12345", "13332.00"},
		{"3", "2026-05-02", "2026-04-30", "", "INV-05", "DE123456789", "13332.00"},
		{"4", "2027-01-04", "2026-12-31", "", "INV-12", "", "13332.00"},
	}
	for i, w := range want {
		r := recs[i+1]
		got := []string{r[0], r[1], r[2], r[3], r[4], r[5], r[10]}
		if strings.Join(got, "|") != strings.Join(w, "|") || r[15] != r[10] || r[6] != "" {
			t.Errorf("row %d: got %q", i+1, r)
		}
	}
	if !strings.HasPrefix(recs[1][16], "EUR 3333.00 x 4") {
		t.Errorf("note: %q", recs[1][16])
	}
}

func TestEwidJPK(t *testing.T) {
	cfg, _ := ewidArchive(t)
	out, err := run(t, cmdEwid, "-c", cfg, "--jpk", "2026")
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"<CelZlozenia>1</CelZlozenia>", "<K_5>5260250274-20260401-ABCDEF123456-AB</K_5>",
		"<K_6>DE</K_6>\n    <K_7>HRB 12345</K_7>", "<K_7>123456789</K_7>", "<LiczbaWierszy>4</LiczbaWierszy>",
		"<SumaPrzychodow>53328.00</SumaPrzychodow>", "<etd:NIP>5260250274</etd:NIP>"} {
		if !strings.Contains(out, want) {
			t.Errorf("JPK lacks %q", want)
		}
	}
	if fixed, _ := run(t, cmdEwid, "-c", cfg, "--jpk", "--correction", "2026"); !strings.Contains(fixed, "<CelZlozenia>2</CelZlozenia>") {
		t.Error("--correction must give CelZlozenia 2")
	}

	// against the official schema, when xmllint is there to check it
	if _, err := exec.LookPath("xmllint"); err != nil {
		t.Skip("xmllint not installed; schema not checked")
	}
	f := filepath.Join(t.TempDir(), "jpk.xml")
	mustWrite(t, f, []byte(out))
	if b, err := exec.Command("xmllint", "--noout", "--nonet", "--schema", "testdata/jpk_ewp4/Schemat_JPK_EWP(4)_v1-0.xsd", f).CombinedOutput(); err != nil {
		t.Errorf("JPK_EWP does not validate:\n%s\n%s", b, out)
	}
}

func TestEwidRefusals(t *testing.T) {
	cfg, root := ewidArchive(t)
	noJPK := filepath.Join(t.TempDir(), "r.conf")
	mustWrite(t, noJPK, []byte(strings.Replace(validConfig, "/srv/invoices", root, 1)))
	var exit exitError
	if _, err := run(t, cmdEwid, "-c", noJPK, "--jpk", "2026"); !errors.As(err, &exit) || exit.code != 2 ||
		!strings.Contains(err.Error(), "birth_date, first_name, surname, tax_office") {
		t.Errorf("--jpk without [jpk]: %v", err)
	}
	for _, args := range [][]string{{"--csv", "--jpk"}, {"--correction"}, {"26"}} {
		if _, err := run(t, cmdEwid, append([]string{"-c", cfg}, args...)...); !errors.As(err, &exit) || exit.code != 2 {
			t.Errorf("%v: want exit 2, got %v", args, err)
		}
	}
	if out, _ := run(t, cmdEwid, "-c", cfg, "2025"); !strings.Contains(out, "No revenue in the year") {
		t.Errorf("empty year: %s", out)
	}
}
