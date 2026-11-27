package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// An Invoify export in the shape the archive has, with placeholder data.
const invoifyTemplate = `{"sender":{"name":"Seller","customInputs":[{"key":"NIP","value":"5260250274"}]},
"receiver":{"name":"Client GmbH","country":"Germany"},
"details":{"invoiceLogo":"data:image/png;base64,AAAA","invoiceNumber":"DEV-2026-10-001",
"invoiceDate":"2026-10-01T00:00:00.000Z","dueDate":"2026-10-08T00:00:00.000Z","currency":"EUR",
"items":[{"name":"Software Engineering","description":"Amount in PLN calculated at the National Bank of Poland exchange rate on 29.09.2026: 14574.54 PLN","quantity":"1","unitPrice":"3333","total":"3333.00"}],
"additionalNotes":"Phase 2 of the project.","paymentTerms":"This invoice covers services rendered from 01 September 2026 to 30 September 2026.\nPayment due within 7 days.",
"subTotal":3333,"totalAmount":3333,"pdfTemplate":2,"signature":{"data":"x"}}}`

func TestNew(t *testing.T) {
	var calls []string
	fakeNBP(t, &calls) // its last table is 212/A of 2026-10-30, 4.3955
	root := writeArchive(t, map[string]string{"2026-10/invoice.json": invoifyTemplate})
	cfg := filepath.Join(t.TempDir(), "ryczalt.conf")
	mustWrite(t, cfg, []byte(strings.Replace(validConfig, "/srv/invoices", root, 1)))
	t.Setenv("XDG_CACHE_HOME", t.TempDir())

	stdout := os.Stdout
	os.Stdout, _ = os.OpenFile(os.DevNull, os.O_WRONLY, 0)
	t.Cleanup(func() { _ = os.Stdout.Close(); os.Stdout = stdout })

	// flags after the month work too
	if err := cmdNew([]string{"-c", cfg, "2026-10", "--date", "2026-11-02"}); err != nil {
		t.Fatal(err)
	}

	b, err := os.ReadFile(filepath.Join(root, "2026-11", "invoice.json"))
	if err != nil {
		t.Fatal(err)
	}

	var got struct {
		Sender  map[string]any
		Details map[string]any
	}
	if err := json.Unmarshal(b, &got); err != nil {
		t.Fatal(err)
	}

	d := got.Details
	item := d["items"].([]any)[0].(map[string]any)
	checks := map[string]any{
		"number":   d["invoiceNumber"] == "DEV-2026-11-001",
		"issued":   d["invoiceDate"] == "2026-11-02T00:00:00.000Z",
		"due":      d["dueDate"] == "2026-11-09T00:00:00.000Z", // payment_days=7 from Mon 2 Nov
		"rate":     item["description"] == "Amount in PLN calculated at the National Bank of Poland exchange rate on 30.10.2026: 14650.20 PLN",
		"period":   strings.HasPrefix(d["paymentTerms"].(string), "This invoice covers services rendered from 01 October 2026 to 31 October 2026.\nPayment"),
		"notes":    d["additionalNotes"] == "Phase 2 of the project.",
		"logo":     d["invoiceLogo"] == "data:image/png;base64,AAAA",
		"total":    string(must(json.Marshal(d["totalAmount"]))) == "3333",
		"sender":   got.Sender["name"] == "Seller",
		"rate day": len(calls) > 0 && calls[0] == "/eur/2026-10-20/2026-10-30/", // last business day before 31 Oct
	}

	for name, ok := range checks {
		if ok != true {
			t.Errorf("%s wrong:\n%s", name, b)
		}
	}

	if err := cmdNew([]string{"-c", cfg, "--date", "2026-11-02", "2026-10"}); err == nil || !strings.Contains(err.Error(), "already exists") {
		t.Errorf("second run: %v", err)
	}

	// issued Sun 1 Nov: the 7th day is Sun 8 Nov, so the deadline is Mon 9 Nov
	if err := os.Remove(filepath.Join(root, "2026-11", "invoice.json")); err != nil {
		t.Fatal(err)
	}

	if err := cmdNew([]string{"-c", cfg, "--date", "2026-11-01", "2026-10"}); err != nil {
		t.Fatal(err)
	}

	b, _ = os.ReadFile(filepath.Join(root, "2026-11", "invoice.json"))
	if !strings.Contains(string(b), `"dueDate":"2026-11-09T00:00:00.000Z"`) {
		t.Errorf("due date not moved past Sunday:\n%s", b)
	}

	if err := cmdNew([]string{"-c", cfg, "--date", "2026-10-15", "2026-10"}); err == nil || !strings.Contains(err.Error(), "before the work ends") {
		t.Errorf("issued mid-month: %v", err)
	}
}

func must(b []byte, err error) []byte {
	if err != nil {
		panic(err)
	}
	return b
}

func TestNextInvoifyWithoutSentences(t *testing.T) {
	tmpl := strings.NewReplacer("exchange rate on 29.09.2026: 14574.54 PLN", "see the rate", "from 01 September 2026 to 30 September 2026", "for September").Replace(invoifyTemplate)
	_, notes, err := nextInvoify([]byte(tmpl), "X", day(t, "2026-11-02"), day(t, "2026-11-09"),
		day(t, "2026-10-01"), day(t, "2026-10-31"), nbpRate{Mid: "4.3955", Table: "212/A/NBP/2026", Date: "2026-10-30"}, 1465020)
	if err != nil || len(notes) != 2 {
		t.Errorf("want two notes to edit by hand, got %q %v", notes, err)
	}
}

func TestNewRefusesIssuedNumber(t *testing.T) {
	var calls []string
	fakeNBP(t, &calls)

	// October's invoice already sent as XML, its invoice.json moved away
	root := writeArchive(t, map[string]string{
		"2026-09/invoice.json":             strings.NewReplacer("DEV-2026-10-001", "DEV-2026-09-001", "2026-10-01T", "2026-09-01T").Replace(invoifyTemplate),
		"2026-10/ksef-DEV-2026-10-001.xml": fa("DEV-2026-10-001", "2026-10-01", "2026-09-30"),
	})

	cfg := filepath.Join(t.TempDir(), "ryczalt.conf")
	mustWrite(t, cfg, []byte(strings.Replace(validConfig, "/srv/invoices", root, 1)))
	t.Setenv("XDG_CACHE_HOME", t.TempDir())

	if _, err := run(t, cmdNew, "-c", cfg, "2026-09", "--date", "2026-10-01"); err == nil || !strings.Contains(err.Error(), "DEV-2026-10-001 already exists") {
		t.Errorf("got %v", err)
	}
}
