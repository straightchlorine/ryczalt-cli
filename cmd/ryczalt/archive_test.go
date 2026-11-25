package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const testNIP = "5260250274"

// faFrom is an FA(3) invoice with placeholder data; extra goes inside <Fa>.
func faFrom(seller, number, issued, p6, extra string) string {
	period := "<P_6>" + p6 + "</P_6>"
	if from, to, ok := strings.Cut(p6, ".."); ok {
		period = "<OkresFa><P_6_Od>" + from + "</P_6_Od><P_6_Do>" + to + "</P_6_Do></OkresFa>"
	}
	return `<?xml version="1.0" encoding="UTF-8"?>
<Faktura xmlns="http://crd.gov.pl/wzor/2025/06/25/13775/">
<Podmiot1><DaneIdentyfikacyjne><NIP>` + seller + `</NIP></DaneIdentyfikacyjne></Podmiot1>
<Fa><KodWaluty>EUR</KodWaluty><P_1>` + issued + `</P_1><P_2>` + number + `</P_2>` + period +
		`<P_13_7>3333.00</P_13_7><P_15>3333.00</P_15>` + extra + `<RodzajFaktury>VAT</RodzajFaktury></Fa></Faktura>`
}

func fa(number, issued, p6 string) string { return faFrom(testNIP, number, issued, p6, "") }

func upo(docs ...[2]string) string {
	s := `<Potwierdzenie xmlns="http://upo.schematy.mf.gov.pl/KSeF/v4-3">`
	for _, d := range docs {
		s += `<Dokument><NumerKSeFDokumentu>` + d[1] + `</NumerKSeFDokumentu><NumerFaktury>` + d[0] +
			`</NumerFaktury><TrybWysylki>Offline</TrybWysylki></Dokument>`
	}
	return s + `</Potwierdzenie>`
}

func invoify(number, date string) string {
	return `{"details":{"invoiceNumber":"` + number + `","invoiceDate":"` + date + `","currency":"eur","totalAmount":3333}}`
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

func summary(got []invoice) string {
	var lines []string
	for _, i := range got {
		lines = append(lines, strings.Join([]string{i.number, i.issued.Format("2006-01-02"),
			i.serviceEnd.Format("2006-01-02"), i.currency, i.total.FloatString(2), i.source, i.ksefNumber, i.ksefMode}, " "))
	}
	return strings.Join(lines, "\n")
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
		// a draft whose total differs from what was sent
		"2026-07/invoice.json":    strings.Replace(invoify("INV-07", "2026-07-01T00:00:00.000Z"), "3333}", "3300}", 1),
		"2026-07/ksef-INV-07.xml": fa("INV-07", "2026-07-01", "2026-06-30"),
		// bash ksef tool: .ksef/ with the XML twice and the UPO
		"2026-06/invoice.json":        invoify("INV-06", "2026-06-01T00:00:00.000Z"),
		"2026-06/.ksef/fa3.xml":       fa("INV-06", "2026-06-01", "2026-05-31"),
		"2026-06/.ksef/ksef-copy.xml": strings.Replace(fa("INV-06", "2026-06-01", "2026-05-31"), "3333.00", "3333", 2),
		"2026-06/.ksef/meta.json":     `{"invoice_number":"INV-06","ksef_number":"KSEF-06"}`,
		"2026-06/.ksef/upo.xml":       upo([2]string{"INV-06", ""}), // an empty number keeps meta.json's
		// ksef-cli: XML and UPO side by side
		"2026-11/ksef-INV-11.xml":     fa("INV-11", "2026-11-01", "2026-10-31"),
		"2026-11/ksef-INV-11.upo.xml": upo([2]string{"INV-11", "KSEF-11"}),
		// a purchase invoice pulled from KSeF, and a file that is not XML
		"2026-11/purchase.xml":    faFrom("1234563218", "THEIRS-1", "2026-11-03", "2026-11-03", ""),
		"2026-11/a212z261030.xml": `<?xml version="1.0" encoding="ISO-8859-2"?><tabela_kursow typ="A"/>`,
		"2026-11/notes.xml":       "not xml at all",
	})
	got, found, err := readArchive(root, testNIP)
	if err != nil {
		t.Fatal(err)
	}
	want := strings.Join([]string{
		"INV-03 2026-03-01 2026-02-28 EUR 3333.00 2026-03/invoice.json  ",
		"INV-04 2026-04-02 2026-03-31 EUR 3333.00 2026-05/KSEF-04.xml KSEF-04 ",
		"INV-05 2026-05-23 2026-04-30 EUR 3333.00 2026-05/KSEF-05.xml  ",
		"INV-06 2026-06-01 2026-05-31 EUR 3333.00 2026-06/.ksef/fa3.xml KSEF-06 Offline",
		"INV-07 2026-07-01 2026-06-30 EUR 3333.00 2026-07/ksef-INV-07.xml  ",
		"INV-11 2026-11-01 2026-10-31 EUR 3333.00 2026-11/ksef-INV-11.xml KSEF-11 Offline",
	}, "\n")
	if got := summary(got); got != want {
		t.Errorf("got\n%s\nwant\n%s", got, want)
	}
	wantFound := strings.Join([]string{
		"warn archive 2026-01/invoice.json: no invoice number or date; skipped",
		"check security 2026-04/.ksef/meta.json: KSeF session key in clear text; delete aes_key_hex and aes_iv_hex",
		"warn archive 2026-11/notes.xml: not readable XML (EOF); skipped",
		"info archive 2026-11/purchase.xml: invoice THEIRS-1 was issued by NIP 1234563218, not by you; skipped",
		"info drafts INV-04: 2026-04/invoice.json is dated 2026-04-01, the XML sent to KSeF 2026-04-02",
		"warn archive INV-07: 2026-07/invoice.json differs from the XML sent to KSeF (in 2026-07) in total (3300.00 vs 3333.00); using the XML",
	}, "\n")
	if got := findingsText(found); got != wantFound {
		t.Errorf("findings:\n%s\nwant\n%s", got, wantFound)
	}
}

func findingsText(fs []finding) string {
	var lines []string
	for _, f := range fs {
		lines = append(lines, f.level+" "+f.group+" "+f.text)
	}
	return strings.Join(lines, "\n")
}

func TestCheckInvoices(t *testing.T) {
	inv := func(number, issued, end, ksefNumber, mode string) invoice {
		return invoice{number: number, issued: day(t, issued), serviceEnd: day(t, end), ksefNumber: ksefNumber, ksefMode: mode}
	}
	got := findingsText(checkInvoices([]invoice{
		inv("OLD", "2026-03-02", "2026-02-28", "", ""), // before KSeF: nothing to check
		inv("NONE", "2026-04-01", "2026-03-31", "", ""),
		inv("META", "2026-05-23", "2026-04-30", "K", ""),
		inv("OFF", "2026-06-01", "2026-05-31", "K", "Offline"),
		inv("ON", "2026-07-01", "2026-06-30", "K", "Online"),
	}))
	want := strings.Join([]string{
		"check KSeF NONE: no KSeF number in the archive (UPO or .ksef/meta.json)",
		"info KSeF META: no UPO kept, the KSeF number comes from .ksef/meta.json",
		"check invoicing META: issued 2026-05-23, after the deadline 2026-05-15 (art. 106i ust. 1 ustawy o VAT)",
		"info KSeF OFF: KSeF recorded it as Offline, sent after its issue date",
	}, "\n")
	if got != want {
		t.Errorf("got\n%s\nwant\n%s", got, want)
	}
}

func TestReadArchiveEdges(t *testing.T) {
	// a UPO for several invoices, same-day invoices in a fixed order, and a
	// month folder that is a symlink
	root := writeArchive(t, map[string]string{
		"2026-06/b.xml":   fa("INV-B", "2026-06-01", "2026-05-31"),
		"2026-06/a.xml":   fa("INV-A", "2026-06-01", "2026-05-31"),
		"2026-06/upo.xml": upo([2]string{"INV-A", "K-A"}, [2]string{"INV-B", "K-B"}),
		"elsewhere/c.xml": fa("INV-C", "2026-07-01", "2026-06-30"),
	})
	if err := os.Symlink(filepath.Join(root, "elsewhere"), filepath.Join(root, "2026-07")); err != nil {
		t.Fatal(err)
	}
	got, found, err := readArchive(root, testNIP)
	if err != nil || len(found) > 0 {
		t.Fatal(err, found)
	}
	want := "INV-A 2026-06-01 2026-05-31 EUR 3333.00 2026-06/a.xml K-A Offline\n" +
		"INV-B 2026-06-01 2026-05-31 EUR 3333.00 2026-06/b.xml K-B Offline\n" +
		"INV-C 2026-07-01 2026-06-30 EUR 3333.00 2026-07/c.xml  "
	if s := summary(got); s != want {
		t.Errorf("got\n%s\nwant\n%s", s, want)
	}
}

func TestReadArchiveErrors(t *testing.T) {
	cases := map[string]struct {
		files map[string]string
		want  string
	}{
		"two XMLs differ": {map[string]string{
			"2026-06/.ksef/fa3.xml":   fa("INV-06", "2026-06-01", "2026-05-31"),
			"2026-07/ksef-INV-06.xml": fa("INV-06", "2026-06-02", "2026-05-31"),
		}, "are both invoice INV-06 but differ in issue date"},
		"two invoice.json differ": {map[string]string{
			"2026-03/invoice.json": invoify("INV-1", "2026-03-05T00:00:00.000Z"),
			"2026-04/invoice.json": invoify("INV-1", "2026-04-05T00:00:00.000Z"),
		}, "are both invoice INV-1 but differ in issue date (2026-03-05 vs 2026-04-05), service end"},
		"correction": {map[string]string{
			"2026-06/k.xml": strings.Replace(fa("KOR-1", "2026-06-01", "2026-05-31"), ">VAT<", ">KOR<", 1),
		}, `invoice KOR-1 is of kind "KOR"`},
		"VAT charged": {map[string]string{
			"2026-06/v.xml": faFrom(testNIP, "INV-V", "2026-06-01", "2026-05-31", "<P_14_1>23.00</P_14_1>"),
		}, "invoice INV-V charges VAT (P_14_1)"},
		"no currency": {map[string]string{
			"2026-06/c.xml": strings.Replace(fa("INV-C", "2026-06-01", "2026-05-31"), "EUR", "", 1),
		}, "without a readable"},
	}
	for name, c := range cases {
		_, _, err := readArchive(writeArchive(t, c.files), testNIP)
		if err == nil || !strings.Contains(err.Error(), c.want) {
			t.Errorf("%s: got %v, want %q", name, err, c.want)
		}
	}

	// identical copies, and a zero VAT amount, are fine
	root := writeArchive(t, map[string]string{
		"2026-03/invoice.json": invoify("INV-1", "2026-03-05T00:00:00.000Z"),
		"2026-04/invoice.json": invoify("INV-1", "2026-03-05T00:00:00.000Z"),
		"2026-06/v.xml":        faFrom(testNIP, "INV-V", "2026-06-01", "2026-05-31", "<P_14_1>0.00</P_14_1>"),
	})
	if got, _, err := readArchive(root, testNIP); err != nil || len(got) != 2 {
		t.Errorf("got %d invoices, %v", len(got), err)
	}
}

// Half-written files are skipped with a warning (a copy still syncing);
// anything another seller issued is skipped whatever it holds.
func TestReadArchiveSkips(t *testing.T) {
	theirs := faFrom("1234563218", "P-1", "2026-06-01", "2026-05-31", "<P_14_1>230.00</P_14_1>")
	root := writeArchive(t, map[string]string{
		"2026-06/t.xml":        "<Faktura><Fa>",
		"2026-06/l.xml":        `<?xml version="1.0" encoding="ISO-8859-2"?><Faktura><Fa/></Faktura>`,
		"2026-06/vat.xml":      theirs,
		"2026-06/kor.xml":      strings.Replace(faFrom("1234563218", "P-2", "2026-06-01", "2026-05-31", ""), ">VAT<", ">KOR<", 1),
		"2026-06/mine.xml":     fa("INV-06", "2026-06-01", "2026-05-31"),
		"2026-06/nothing.json": "{}",
	})
	got, found, err := readArchive(root, testNIP)
	if err != nil || len(got) != 1 {
		t.Fatalf("got %d invoices, %v", len(got), err)
	}
	text := findingsText(found)
	for _, want := range []string{"warn archive 2026-06/l.xml: not readable XML", "warn archive 2026-06/t.xml: not readable XML",
		"info archive 2026-06/kor.xml: invoice P-2 was issued by NIP 1234563218", "info archive 2026-06/vat.xml: invoice P-1 was issued by NIP 1234563218"} {
		if !strings.Contains(text, want) {
			t.Errorf("missing %q in\n%s", want, text)
		}
	}
}
