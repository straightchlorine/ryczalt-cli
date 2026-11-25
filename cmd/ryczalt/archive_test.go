package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func fa(number, issued, p6 string) string {
	period := "<P_6>" + p6 + "</P_6>"
	if from, to, ok := strings.Cut(p6, ".."); ok {
		period = "<OkresFa><P_6_Od>" + from + "</P_6_Od><P_6_Do>" + to + "</P_6_Do></OkresFa>"
	}
	return `<?xml version="1.0" encoding="UTF-8"?>
<Faktura xmlns="http://crd.gov.pl/wzor/2025/06/25/13775/"><Fa><KodWaluty>EUR</KodWaluty>
<P_1>` + issued + `</P_1><P_2>` + number + `</P_2>` + period + `<P_15>3333.00</P_15></Fa></Faktura>`
}

func upo(number, ksefNumber string) string {
	return `<Potwierdzenie xmlns="http://upo.schematy.mf.gov.pl/KSeF/v4-3"><Dokument>
<NumerKSeFDokumentu>` + ksefNumber + `</NumerKSeFDokumentu><NumerFaktury>` + number + `</NumerFaktury>
<TrybWysylki>Offline</TrybWysylki></Dokument></Potwierdzenie>`
}

func invoify(number, date string) string {
	return `{"details":{"invoiceNumber":"` + number + `","invoiceDate":"` + date + `","currency":"EUR","totalAmount":3333}}`
}

func writeArchive(t *testing.T, files map[string]string) string {
	t.Helper()
	root := t.TempDir()
	for name, body := range files {
		p := filepath.Join(root, name)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

// The layouts the bash tools left in the real archive, with placeholder data.
func TestReadArchive(t *testing.T) {
	root := writeArchive(t, map[string]string{
		// before KSeF: Invoify only; UTC midnight before is still the 1st in Warsaw
		"2026-03/invoice.json":          invoify("INV-03", "2026-02-28T23:00:00.000Z"),
		"2026-03/invoice-mine.json":     invoify("INV-03", "2026-03-05T00:00:00.000Z"),
		"2026-03/a050z260313.xml":       `<tabela_kursow typ="A"></tabela_kursow>`,
		"2026-01/invoice.json":          `{"details":{"invoiceNumber":"INV-01","invoiceDate":null}}`,
		"tax-logs/2026-03/invoice.json": invoify("NOT-AN-INVOICE", "2026-03-01T00:00:00.000Z"),
		// sent by hand: XML named by KSeF number, sitting in the next month's folder
		"2026-04/invoice.json":    invoify("INV-04", "2026-04-01T00:00:00.000Z"),
		"2026-04/.ksef/meta.json": `{"invoice_number":"INV-04","ksef_number":"KSEF-04","aes_key_hex":"00"}`,
		"2026-05/KSEF-04.xml":     fa("INV-04", "2026-04-02", "2026-03-31"),
		"2026-05/KSEF-05.xml":     fa("INV-05", "2026-05-23", "2026-04-01..2026-04-30"),
		// bash ksef tool: .ksef/ with the XML twice and the UPO
		"2026-06/invoice.json":        invoify("INV-06", "2026-06-01T00:00:00.000Z"),
		"2026-06/.ksef/fa3.xml":       fa("INV-06", "2026-06-01", "2026-05-31"),
		"2026-06/.ksef/ksef-copy.xml": strings.Replace(fa("INV-06", "2026-06-01", "2026-05-31"), "3333.00", "3333", 1),
		"2026-06/.ksef/upo.xml":       upo("INV-06", "KSEF-06"),
		// ksef-cli: XML and UPO side by side
		"2026-11/ksef-INV-11.xml":     fa("INV-11", "2026-11-01", "2026-10-31"),
		"2026-11/ksef-INV-11.upo.xml": upo("INV-11", "KSEF-11"),
	})
	got, warns, err := readArchive(root)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{
		"INV-03 2026-03-01 2026-02-28 EUR 3333.00 2026-03/invoice.json  ",
		"INV-04 2026-04-02 2026-03-31 EUR 3333.00 2026-05/KSEF-04.xml KSEF-04 ",
		"INV-05 2026-05-23 2026-04-30 EUR 3333.00 2026-05/KSEF-05.xml  ",
		"INV-06 2026-06-01 2026-05-31 EUR 3333.00 2026-06/.ksef/fa3.xml KSEF-06 Offline",
		"INV-11 2026-11-01 2026-10-31 EUR 3333.00 2026-11/ksef-INV-11.xml KSEF-11 Offline",
	}
	var lines []string
	for _, i := range got {
		lines = append(lines, strings.Join([]string{i.number, i.issued.Format("2006-01-02"), i.serviceEnd.Format("2006-01-02"),
			i.currency, i.total.FloatString(2), i.source, i.ksefNumber, i.ksefMode}, " "))
	}
	if strings.Join(lines, "\n") != strings.Join(want, "\n") {
		t.Errorf("got\n%s\nwant\n%s", strings.Join(lines, "\n"), strings.Join(want, "\n"))
	}
	wantWarns := []string{
		"2026-01/invoice.json: no invoice number or date; skipped",
		"INV-04: 2026-04/invoice.json differs from the sent 2026-05/KSEF-04.xml in issue date (2026-04-01 vs 2026-04-02); using the sent XML",
	}
	if strings.Join(warns, "\n") != strings.Join(wantWarns, "\n") {
		t.Errorf("warnings:\n%s", strings.Join(warns, "\n"))
	}
}

func TestReadArchiveConflict(t *testing.T) {
	root := writeArchive(t, map[string]string{
		"2026-06/.ksef/fa3.xml":   fa("INV-06", "2026-06-01", "2026-05-31"),
		"2026-07/ksef-INV-06.xml": fa("INV-06", "2026-06-02", "2026-05-31"),
	})
	_, _, err := readArchive(root)
	if err == nil || !strings.Contains(err.Error(), "are both invoice INV-06 but differ in issue date") {
		t.Errorf("got %v", err)
	}
}
