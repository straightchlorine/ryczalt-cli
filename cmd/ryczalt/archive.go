package main

import (
	"bytes"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io/fs"
	"math/big"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"time"
	_ "time/tzdata" // Polish calendar days on any OS
)

// warsaw is where invoice dates are calendar days.
var warsaw = func() *time.Location {
	l, err := time.LoadLocation("Europe/Warsaw")
	if err != nil {
		panic(err)
	}
	return l
}()

// invoice is one issued invoice as the archive records it.
type invoice struct {
	folder     string    // YYYY-MM folder it was found in
	source     string    // file the data comes from, relative to the archive
	number     string    // P_2
	issued     time.Time // P_1
	serviceEnd time.Time // P_6_Do or P_6; for invoice.json the last day of the month before issue
	currency   string    // KodWaluty
	total      *big.Rat  // P_15, gross
	ksefNumber string    // from the UPO or .ksef/meta.json; "" if never sent
	ksefMode   string    // TrybWysylki from the UPO: Online or Offline
}

var folderName = regexp.MustCompile(`^\d{4}-(0[1-9]|1[0-2])$`)

// readArchive reads the invoices in every YYYY-MM folder under root.
// Includes layouts other tools might have left:
// - FA(3) XML anywhere in the folder
// - Invoify invoice.json for invoices that didn't go to KSeF
// - compliance to authors older bash tools (not much thought put into them)
func readArchive(root string) ([]invoice, []string, error) {
	dirs, err := os.ReadDir(root)
	if err != nil {
		return nil, nil, err
	}
	sent := map[string]invoice{}  // number -> invoice from FA(3) XML
	draft := map[string]invoice{} // number -> invoice from invoice.json
	type ksef struct{ number, mode string }
	ksefOf := map[string]ksef{}
	var warns []string

	for _, d := range dirs {
		if !d.IsDir() || !folderName.MatchString(d.Name()) {
			continue
		}
		folder := d.Name()
		err := filepath.WalkDir(
			filepath.Join(root, folder),
			func(path string, e fs.DirEntry, err error) error {
				if err != nil || e.IsDir() {
					return err
				}
				rel, _ := filepath.Rel(root, path)
				name := e.Name()
				if !strings.HasSuffix(name, ".xml") && name != "invoice.json" &&
					name != "meta.json" {
					return nil
				}
				b, err := os.ReadFile(path)
				if err != nil {
					return err
				}
				switch {
				case name == "invoice.json" && filepath.Dir(rel) == folder:
					inv, err := readInvoify(b)
					if err != nil {
						warns = append(warns, fmt.Sprintf("%s: %v; skipped", rel, err))
						return nil
					}
					inv.folder, inv.source = folder, rel
					draft[inv.number] = inv
				case name == "meta.json" && filepath.Base(filepath.Dir(rel)) == ".ksef":
					var m struct{ Invoice_number, Ksef_number string }
					if err := json.Unmarshal(b, &m); err != nil {
						return fmt.Errorf("%s: %w", rel, err)
					}
					if m.Ksef_number != "" && ksefOf[m.Invoice_number].number == "" {
						ksefOf[m.Invoice_number] = ksef{number: m.Ksef_number}
					}
				case strings.HasSuffix(name, ".xml"):
					switch xmlRoot(b) {
					case "Faktura":
						inv, err := readFA(b)
						if err != nil {
							return fmt.Errorf("%s: %w", rel, err)
						}
						inv.folder, inv.source = folder, rel
						if prev, ok := sent[inv.number]; ok {
							if diff := differ(prev, inv); diff != "" {
								return fmt.Errorf(
									"%s and %s are both invoice %s but differ in %s",
									prev.source,
									rel,
									inv.number,
									diff,
								)
							}
							return nil
						}
						sent[inv.number] = inv
					case "Potwierdzenie":
						var u struct {
							Number string `xml:"Dokument>NumerFaktury"`
							KSeF   string `xml:"Dokument>NumerKSeFDokumentu"`
							Mode   string `xml:"Dokument>TrybWysylki"`
						}
						if err := xml.Unmarshal(b, &u); err != nil {
							return fmt.Errorf("%s: %w", rel, err)
						}
						ksefOf[u.Number] = ksef{u.KSeF, u.Mode}
					}
				}
				return nil
			},
		)
		if err != nil {
			return nil, nil, err
		}
	}

	var out []invoice
	for n, inv := range draft {
		if s, ok := sent[n]; ok {
			if diff := differ(inv, s); diff != "" {
				warns = append(
					warns,
					fmt.Sprintf(
						"%s: %s differs from the sent %s in %s; using the sent XML",
						n,
						inv.source,
						s.source,
						diff,
					),
				)
			}
			continue
		}
		out = append(out, inv)
	}
	for _, inv := range sent {
		out = append(out, inv)
	}
	for i := range out {
		k := ksefOf[out[i].number]
		out[i].ksefNumber, out[i].ksefMode = k.number, k.mode
	}
	slices.SortFunc(out, func(a, b invoice) int { return a.issued.Compare(b.issued) })
	slices.Sort(warns)
	return out, warns, nil
}

// differ names the fields in which two records of one invoice disagree.
func differ(a, b invoice) string {
	var d []string
	if !a.issued.Equal(b.issued) {
		d = append(
			d,
			fmt.Sprintf(
				"issue date (%s vs %s)",
				a.issued.Format(time.DateOnly),
				b.issued.Format(time.DateOnly),
			),
		)
	}
	if !a.serviceEnd.Equal(b.serviceEnd) {
		d = append(
			d,
			fmt.Sprintf(
				"service end (%s vs %s)",
				a.serviceEnd.Format(time.DateOnly),
				b.serviceEnd.Format(time.DateOnly),
			),
		)
	}
	if a.currency != b.currency {
		d = append(d, "currency")
	}
	if a.total.Cmp(b.total) != 0 {
		d = append(d, "total")
	}
	return strings.Join(d, ", ")
}

// xmlRoot is the local name of the document element, or "".
func xmlRoot(b []byte) string {
	dec := xml.NewDecoder(bytes.NewReader(b))
	for {
		t, err := dec.Token()
		if err != nil {
			return ""
		}
		if s, ok := t.(xml.StartElement); ok {
			return s.Name.Local
		}
	}
}

func readFA(b []byte) (invoice, error) {
	var f struct {
		P1       string `xml:"Fa>P_1"`
		P2       string `xml:"Fa>P_2"`
		P6       string `xml:"Fa>P_6"`
		P6Do     string `xml:"Fa>OkresFa>P_6_Do"`
		Currency string `xml:"Fa>KodWaluty"`
		P15      string `xml:"Fa>P_15"`
	}
	if err := xml.Unmarshal(b, &f); err != nil {
		return invoice{}, err
	}
	end := f.P6Do
	if end == "" {
		end = f.P6
	}
	if end == "" {
		end = f.P1 // no separate service date: the issue date is the service date
	}
	issued, err1 := time.Parse(time.DateOnly, f.P1)
	serviceEnd, err2 := time.Parse(time.DateOnly, end)
	total, ok := new(big.Rat).SetString(f.P15)
	if f.P2 == "" || err1 != nil || err2 != nil || !ok {
		return invoice{}, fmt.Errorf("FA(3) without a readable P_1, P_2, P_6 or P_15")
	}
	return invoice{
		number:     f.P2,
		issued:     issued,
		serviceEnd: serviceEnd,
		currency:   f.Currency,
		total:      total,
	}, nil
}

// readInvoify reads an Invoify export. It has no service date, so the
// service is taken to be the calendar month before the issue date, as
// the earlier tools invoiced.
func readInvoify(b []byte) (invoice, error) {
	var j struct {
		Details struct {
			InvoiceNumber string
			InvoiceDate   string
			Currency      string
			TotalAmount   json.Number
		}
	}
	if err := json.Unmarshal(b, &j); err != nil {
		return invoice{}, err
	}
	d := j.Details
	t, err := time.Parse(time.RFC3339, d.InvoiceDate)
	if err != nil || d.InvoiceNumber == "" {
		return invoice{}, fmt.Errorf("no invoice number or date")
	}
	y, m, day := t.In(warsaw).Date()
	issued := time.Date(y, m, day, 0, 0, 0, 0, time.UTC)
	total, ok := new(big.Rat).SetString(d.TotalAmount.String())
	if !ok {
		return invoice{}, fmt.Errorf("no total amount")
	}
	return invoice{
		number:     d.InvoiceNumber,
		issued:     issued,
		serviceEnd: time.Date(y, m, 0, 0, 0, 0, 0, time.UTC),
		currency:   d.Currency,
		total:      total,
	}, nil
}
