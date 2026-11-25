package main

import (
	"errors"
	"strings"
	"testing"
	"time"
)

const validConfig = `archive=/srv/invoices
[tax]   # podatek
nip=5260250274
form=ryczalt
payment=monthly
vat_status=exempt
rate=12
business_start=2025-06
revenue_basis=period_end   # confirmed
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
		nip:               "5260250274",
		businessStart:     time.Date(2025, 6, 1, 0, 0, 0, 0, time.UTC),
		rate:              "12",
		revenueBasis:      "period_end",
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
		{"unknown key", "chorobowa=yes", "chorobowa=yes\nchorobwa=no", "line 14: unknown key zus.chorobwa", false},
		{"key in wrong section", "chorobowa=yes", "chorobowa=yes\narchive=/srv", "unknown key zus.archive", false},
		{"unknown section", "[zus]", "[vat]", "unknown section [vat]", false},
		{"set twice", "payment=monthly", "payment=monthly\npayment=monthly", "tax.payment is set twice", false},
		{"not key=value", "[zus]", "[zus]\nulga", "not a key=value line", false},
		{"missing", "revenue_basis=period_end   # confirmed", "revenue_basis=", "tax.revenue_basis: required", false},
		{"bad choice", "chorobowa=yes", "chorobowa=tak", `zus.chorobowa: "tak" is not one of yes | no`, false},
		{"bad month", "business_start=2025-06", "business_start=06-2025", "is not a month like 2026-01", false},
		{"phases out of order", "preferential_until=2027-12", "preferential_until=2025-11", "must come after zus.ulga_until", false},
		{"phase before start", "ulga_until=2025-12", "ulga_until=2025-01", "zus.ulga_until: before tax.business_start", false},
		{"relative archive", "archive=/srv/invoices", "archive=invoices", `archive: "invoices" must be an absolute path`, false},
		{"bad NIP", "nip=5260250274", "nip=526-025-02-74", "tax.nip: \"526-025-02-74\" is not a valid NIP", false},
		{"bad NIP check digit", "nip=5260250274", "nip=5260250275", "is not a valid NIP", false},
		{"rate not in the act", "rate=12", "rate=13", `tax.rate: "13" is not one of 17 | 15`, false},
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

func TestConfigByteOrderMark(t *testing.T) {
	if _, err := parseConfig(strings.NewReader("\ufeff" + validConfig)); err != nil {
		t.Error(err)
	}
}
