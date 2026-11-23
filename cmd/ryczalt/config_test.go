package main

import (
	"errors"
	"os"
	"strings"
	"testing"
	"time"
)

const validConfig = `archive=/srv/invoices
[tax]
form=ryczalt
payment=monthly
vat_status=exempt
business_start=2025-06
revenue_basis=period_end   # confirmed
method_confirmed_on="2026-11-20"
[zus]
ulga_until=2025-12
preferential_until=2027-12
chorobowa=yes
`

func TestParseConfig(t *testing.T) {
	c, err := parseConfig(strings.NewReader(validConfig))
	if err != nil {
		t.Fatal(err)
	}
	want := config{
		archive:           "/srv/invoices",
		businessStart:     time.Date(2025, 6, 1, 0, 0, 0, 0, time.UTC),
		revenueBasis:      "period_end",
		methodConfirmedOn: "2026-11-20",
		ulgaUntil:         time.Date(2025, 12, 1, 0, 0, 0, 0, time.UTC),
		preferentialUntil: time.Date(2027, 12, 1, 0, 0, 0, 0, time.UTC),
		chorobowa:         true,
	}
	if c != want {
		t.Errorf("got %+v\nwant %+v", c, want)
	}
}

func TestParseConfigErrors(t *testing.T) {
	cases := []struct {
		name, old, new, want string
		unsupported          bool
	}{
		{"unknown key", "chorobowa=yes", "chorobowa=yes\nchorobwa=no", "line 13: unknown key zus.chorobwa", false},
		{"key in wrong section", "chorobowa=yes", "chorobowa=yes\narchive=/srv", "unknown key zus.archive", false},
		{"unknown section", "[zus]", "[vat]", "unknown section [vat]", false},
		{"set twice", "payment=monthly", "payment=monthly\npayment=monthly", "tax.payment is set twice", false},
		{"not key=value", "[zus]", "[zus]\nulga", "not a key=value line", false},
		{"missing", "revenue_basis=period_end   # confirmed", "revenue_basis=", "tax.revenue_basis: required", false},
		{"bad choice", "chorobowa=yes", "chorobowa=tak", `zus.chorobowa: "tak" is not one of yes | no`, false},
		{"bad month", "business_start=2025-06", "business_start=06-2025", "is not a month like 2026-01", false},
		{"bad date", `"2026-11-20"`, "20.11.2026", "is not a date like 2026-01-31", false},
		{"phases out of order", "preferential_until=2027-12", "preferential_until=2025-11", "must come after zus.ulga_until", false},
		{"phase before start", "ulga_until=2025-12", "ulga_until=2025-01", "zus.ulga_until: before tax.business_start", false},
		{"other tax form", "form=ryczalt", "form=liniowy", "tax.form=liniowy, only ryczalt is supported", true},
		{"active VAT", "vat_status=exempt", "vat_status=active", "only exempt is supported", true},
	}
	for _, c := range cases {
		in := strings.Replace(validConfig, c.old, c.new, 1)
		if in == validConfig {
			t.Fatalf("%s: %q not found in validConfig", c.name, c.old)
		}
		_, err := parseConfig(strings.NewReader(in))
		if err == nil || !strings.Contains(err.Error(), c.want) {
			t.Errorf("%s: got %v, want %q", c.name, err, c.want)
		}
		if errors.Is(err, errUnsupported) != c.unsupported {
			t.Errorf("%s: errors.Is(err, errUnsupported) = %v", c.name, !c.unsupported)
		}
	}
}

func TestConfigExample(t *testing.T) {
	b, err := os.ReadFile("../../ryczalt.conf.example")
	if err != nil {
		t.Fatal(err)
	}
	// The example leaves the accountant's answer blank on purpose.
	_, err = parseConfig(strings.NewReader(string(b)))
	if err == nil || err.Error() != "tax.revenue_basis: required" {
		t.Fatalf("example: got %v, want only the blank revenue_basis", err)
	}
	filled := strings.Replace(string(b), "revenue_basis=", "revenue_basis=service", 1)
	if _, err := parseConfig(strings.NewReader(filled)); err != nil {
		t.Errorf("example with revenue_basis filled: %v", err)
	}
}
