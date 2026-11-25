package main

import (
	"bytes"
	"cmp"
	"encoding/json"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
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
	currency   string    // KodWaluty, upper case
	total      *big.Rat  // P_15; with no VAT (checked) it is also the net revenue
	ksefNumber string    // from the UPO or .ksef/meta.json; "" if never sent
	ksefMode   string    // TrybWysylki from the UPO: Online or Offline
	buyerCode  string    // country of the buyer's tax id, e.g. DE; "" if unknown
	buyerID    string    // the buyer's tax id without the country prefix; "" if none
	buyerVAT   bool      // buyerID is a NIP or an EU VAT number, not some other id (NrID)
}

// buyerText is the buyer's id as people write it: DE123456789 for a VAT
// number, "DE HRB 12345" for another kind of id.
func (inv invoice) buyerText() string {
	switch {
	case inv.buyerID == "":
		return ""
	case inv.buyerVAT || inv.buyerCode == "":
		return inv.buyerCode + inv.buyerID
	}
	return inv.buyerCode + " " + inv.buyerID
}

var (
	folderName   = regexp.MustCompile(`^\d{4}-(0[1-9]|1[0-2])$`)
	currencyCode = regexp.MustCompile(`^[A-Z]{3}$`)
)

// maxFileSize skips anything bigger than an invoice could be, so a stray
// video in a month folder is not read into memory.
const maxFileSize = 10 << 20

// readArchive reads the invoices in every YYYY-MM folder under root.
// Includes layouts other tools might have left:
// - FA(3) XML anywhere in the folder
// - Invoify invoice.json for invoices that didn't go to KSeF
// - compliance to authors older bash tools (not much thought put into them)
//
// Only invoices whose seller is nip count; others (e.g. purchase invoices
// pulled from KSeF) are skipped with a warning. Anything that would make
// the revenue wrong without notice is either an error or a warning.
func readArchive(root, nip string) ([]invoice, []finding, error) {
	dirs, err := os.ReadDir(root)
	if err != nil {
		return nil, nil, err
	}
	sent := map[string]invoice{}  // number -> invoice from FA(3) XML
	draft := map[string]invoice{} // number -> invoice from invoice.json
	type ksef struct{ number, mode string }
	ksefOf := map[string]ksef{}
	setKSeF := func(number, ksefNumber, mode string) {
		k := ksefOf[number]
		k.number = cmp.Or(ksefNumber, k.number)
		k.mode = cmp.Or(mode, k.mode)
		ksefOf[number] = k
	}
	var found []finding
	// warn: may change the numbers, shown by status; note: only by check
	warn := func(format string, a ...any) {
		found = append(found, finding{levelWarn, "archive", fmt.Sprintf(format, a...)})
	}
	note := func(level, group, format string, a ...any) {
		found = append(found, finding{level, group, fmt.Sprintf(format, a...)})
	}

	for _, d := range dirs {
		if !folderName.MatchString(d.Name()) {
			continue
		}
		folder := d.Name()
		// a month folder may be a symlink to elsewhere; WalkDir does not follow it
		dir, err := filepath.EvalSymlinks(filepath.Join(root, folder))
		if err != nil {
			warn("%s: %v; skipped", folder, err)
			continue
		}
		if st, err := os.Stat(dir); err != nil || !st.IsDir() {
			continue
		}
		err = filepath.WalkDir(dir, func(path string, e fs.DirEntry, err error) error {
			if err != nil || e.IsDir() {
				return err
			}
			r, _ := filepath.Rel(dir, path)
			rel := filepath.Join(folder, r)
			name := e.Name()
			if !strings.HasSuffix(name, ".xml") && name != "invoice.json" && name != "meta.json" {
				return nil
			}
			if info, err := os.Stat(path); err != nil {
				warn("%s: %v; skipped", rel, err)
				return nil
			} else if info.Size() > maxFileSize {
				note(levelInfo, "archive", "%s: larger than %d MB, not an invoice; skipped", rel, maxFileSize>>20)
				return nil
			}
			b, err := os.ReadFile(path)
			if err != nil {
				warn("%s: %v; skipped", rel, err)
				return nil
			}

			switch {
			case name == "invoice.json" && r == "invoice.json":
				inv, err := readInvoify(b)
				if err != nil {
					warn("%s: %v; skipped", rel, err)
					return nil
				}
				inv.folder, inv.source = folder, rel
				if prev, ok := draft[inv.number]; ok {
					if diff := differ(prev, inv); diff != "" {
						return fmt.Errorf("%s and %s are both invoice %s but differ in %s",
							prev.source, rel, inv.number, diff)
					}
					return nil
				}
				draft[inv.number] = inv

			case name == "meta.json" && filepath.Base(filepath.Dir(r)) == ".ksef":
				var m struct{ Invoice_number, Ksef_number, Aes_key_hex string }
				if err := json.Unmarshal(b, &m); err != nil {
					warn("%s: %v; skipped", rel, err)
					return nil
				}
				if m.Aes_key_hex != "" {
					note(
						levelCheck,
						"security",
						"%s: KSeF session key in clear text; delete aes_key_hex and aes_iv_hex",
						rel,
					)
				}
				if k := ksefOf[strings.TrimSpace(m.Invoice_number)]; k.number == "" {
					setKSeF(
						strings.TrimSpace(m.Invoice_number),
						strings.TrimSpace(m.Ksef_number),
						"",
					)
				}

			case strings.HasSuffix(name, ".xml"):
				kind, err := xmlRoot(b)
				if err != nil {
					warn("%s: not readable XML (%v); skipped", rel, err)
					return nil
				}
				switch kind {
				case "Faktura":
					inv, seller, err := readFA(b)
					switch {
					case errors.Is(err, errUnreadable): // e.g. a copy still syncing
						warn("%s: %v; skipped", rel, err)
						return nil
					case seller == "":
						warn("%s: invoice %s names no seller NIP; skipped", rel, inv.number)
						return nil
					case seller != nip: // not yours, whatever it holds
						note(
							levelInfo,
							"archive",
							"%s: invoice %s was issued by NIP %s, not by you; skipped",
							rel,
							inv.number,
							seller,
						)
						return nil
					case err != nil:
						return fmt.Errorf("%s: %w", rel, err)
					}
					inv.folder, inv.source = folder, rel
					if prev, ok := sent[inv.number]; ok {
						if diff := differ(prev, inv); diff != "" {
							return fmt.Errorf("%s and %s are both invoice %s but differ in %s",
								prev.source, rel, inv.number, diff)
						}
						return nil
					}
					sent[inv.number] = inv
				case "Potwierdzenie":
					var u struct {
						Docs []struct {
							Number string `xml:"NumerFaktury"`
							KSeF   string `xml:"NumerKSeFDokumentu"`
							Mode   string `xml:"TrybWysylki"`
						} `xml:"Dokument"`
					}
					if err := xml.Unmarshal(b, &u); err != nil {
						warn("%s: %v; skipped", rel, err)
						return nil
					}
					for _, d := range u.Docs {
						if n := strings.TrimSpace(d.Number); n != "" {
							setKSeF(n, strings.TrimSpace(d.KSeF), strings.TrimSpace(d.Mode))
						}
					}
				}
			}
			return nil
		})
		if err != nil {
			return nil, nil, err
		}
	}

	var out []invoice
	for n, inv := range draft {
		if s, ok := sent[n]; ok {
			// the XML is what was issued; a draft dated a day or two
			// earlier is normal, so only differences in what is taxed count
			draftAsSent := inv
			draftAsSent.issued = s.issued
			if diff := differ(draftAsSent, s); diff != "" {
				warn(
					"%s: %s differs from the XML sent to KSeF (in %s) in %s; using the XML",
					n,
					inv.source,
					s.folder,
					diff,
				)
			} else if !inv.issued.Equal(s.issued) {
				note(levelInfo, "drafts", "%s: %s is dated %s, the XML sent to KSeF %s", n, inv.source,
					inv.issued.Format(time.DateOnly), s.issued.Format(time.DateOnly))
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
	slices.SortFunc(out, func(a, b invoice) int {
		return cmp.Or(a.issued.Compare(b.issued), strings.Compare(a.number, b.number))
	})
	slices.SortStableFunc(found, func(a, b finding) int { return strings.Compare(a.text, b.text) })
	return out, found, nil
}

// differ names the fields in which two records of one invoice disagree.
func differ(a, b invoice) string {
	var d []string
	if !a.issued.Equal(b.issued) {
		d = append(d, fmt.Sprintf("issue date (%s vs %s)",
			a.issued.Format(time.DateOnly), b.issued.Format(time.DateOnly)))
	}
	if !a.serviceEnd.Equal(b.serviceEnd) {
		d = append(d, fmt.Sprintf("service end (%s vs %s)",
			a.serviceEnd.Format(time.DateOnly), b.serviceEnd.Format(time.DateOnly)))
	}
	if a.currency != b.currency {
		d = append(d, fmt.Sprintf("currency (%s vs %s)", a.currency, b.currency))
	}
	if a.total.Cmp(b.total) != 0 {
		d = append(
			d,
			fmt.Sprintf("total (%s vs %s)", a.total.FloatString(2), b.total.FloatString(2)),
		)
	}
	return strings.Join(d, ", ")
}

// xmlRoot is the local name of the document element.
func xmlRoot(b []byte) (string, error) {
	dec := xml.NewDecoder(bytes.NewReader(b))
	dec.CharsetReader = func(_ string, r io.Reader) (io.Reader, error) { return r, nil }
	for {
		t, err := dec.Token()
		if err != nil {
			return "", err
		}
		if s, ok := t.(xml.StartElement); ok {
			return s.Name.Local, nil
		}
	}
}

var errUnreadable = errors.New("not readable XML")

// readFA reads an FA(3) invoice and returns its seller's NIP alongside.
// Only plain invoices (RodzajFaktury VAT) without any VAT amount are taken:
// corrections and advance invoices change revenue in ways not modelled
// yet, and VAT would mean the profile is wrong.
func readFA(b []byte) (invoice, string, error) {
	var f struct {
		Seller string `xml:"Podmiot1>DaneIdentyfikacyjne>NIP"`
		Buyer  struct {
			NIP, KodUE, NrVatUE, KodKraju, NrID string
		} `xml:"Podmiot2>DaneIdentyfikacyjne"`
		Fa struct {
			Kind     string `xml:"RodzajFaktury"`
			P1       string `xml:"P_1"`
			P2       string `xml:"P_2"`
			P6       string `xml:"P_6"`
			P6Do     string `xml:"OkresFa>P_6_Do"`
			Currency string `xml:"KodWaluty"`
			P15      string `xml:"P_15"`
			Other    []struct {
				XMLName xml.Name
				Value   string `xml:",chardata"`
			} `xml:",any"`
		} `xml:"Fa"`
	}
	if err := xml.Unmarshal(b, &f); err != nil {
		return invoice{}, "", fmt.Errorf("%w (%v)", errUnreadable, err)
	}
	fa := f.Fa
	number := strings.TrimSpace(fa.P2)
	seller := strings.TrimSpace(f.Seller)
	partial := invoice{number: number} // enough to name it in an error
	if kind := strings.TrimSpace(fa.Kind); kind != "VAT" {
		return partial, seller, fmt.Errorf("invoice %s is of kind %q; only VAT (a plain invoice) "+
			"is supported, corrections and advance invoices are not yet", number, kind)
	}
	for _, o := range fa.Other {
		if strings.HasPrefix(o.XMLName.Local, "P_14_") {
			if v, ok := new(big.Rat).SetString(strings.TrimSpace(o.Value)); !ok || v.Sign() != 0 {
				return partial, seller, fmt.Errorf("invoice %s charges VAT (%s), but ryczalt "+
					"handles VAT-exempt sellers only", number, o.XMLName.Local)
			}
		}
	}
	end := cmp.Or(strings.TrimSpace(fa.P6Do), strings.TrimSpace(fa.P6),
		strings.TrimSpace(fa.P1)) // no separate service date: the issue date is the service date
	issued, err1 := time.Parse(time.DateOnly, strings.TrimSpace(fa.P1))
	serviceEnd, err2 := time.Parse(time.DateOnly, end)
	total, ok := new(big.Rat).SetString(strings.TrimSpace(fa.P15))
	currency := strings.ToUpper(strings.TrimSpace(fa.Currency))
	if number == "" || err1 != nil || err2 != nil || !ok || !currencyCode.MatchString(currency) {
		return partial, seller, errors.New(
			"FA(3) without a readable P_1, P_2, P_6, P_15 or KodWaluty",
		)
	}
	inv := invoice{
		number:     number,
		issued:     issued,
		serviceEnd: serviceEnd,
		currency:   currency,
		total:      total,
	}
	switch b := f.Buyer; {
	case strings.TrimSpace(b.NIP) != "":
		inv.buyerCode, inv.buyerID, inv.buyerVAT = "PL", strings.TrimSpace(b.NIP), true
	case strings.TrimSpace(b.NrVatUE) != "":
		inv.buyerCode, inv.buyerID, inv.buyerVAT = vatToISO(
			strings.TrimSpace(b.KodUE),
		), strings.TrimSpace(
			b.NrVatUE,
		), true
	case strings.TrimSpace(b.NrID) != "":
		inv.buyerCode, inv.buyerID = strings.TrimSpace(b.KodKraju), strings.TrimSpace(b.NrID)
	}
	return inv, seller, nil
}

// readInvoify reads an Invoify export. It has no service date, so the
// service is taken to be the calendar month before the issue date, as
// the earlier tools invoiced.
func readInvoify(b []byte) (invoice, error) {
	var j struct {
		Receiver struct {
			CustomInputs []struct{ Key, Value string }
		}
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
	number := strings.TrimSpace(d.InvoiceNumber)
	t, err := time.Parse(time.RFC3339, strings.TrimSpace(d.InvoiceDate))
	if err != nil || number == "" {
		return invoice{}, fmt.Errorf("no invoice number or date")
	}
	y, m, day := t.In(warsaw).Date()
	total, ok := new(big.Rat).SetString(d.TotalAmount.String())
	if !ok {
		return invoice{}, fmt.Errorf("no total amount")
	}
	currency := strings.ToUpper(strings.TrimSpace(d.Currency))
	if !currencyCode.MatchString(currency) {
		return invoice{}, fmt.Errorf("currency %q is not a three-letter code", d.Currency)
	}
	inv := invoice{
		number:     number,
		issued:     time.Date(y, m, day, 0, 0, 0, 0, time.UTC),
		serviceEnd: time.Date(y, m, 0, 0, 0, 0, 0, time.UTC),
		currency:   currency,
		total:      total,
	}
	// the buyer's VAT id, as the earlier invoices put it: a custom field
	for _, in := range j.Receiver.CustomInputs {
		k := strings.ToUpper(strings.TrimSpace(in.Key))
		v := strings.ReplaceAll(strings.ToUpper(strings.TrimSpace(in.Value)), " ", "")
		if (k == "VAT" || k == "NRVATUE" || k == "VAT ID" || k == "NIP") && v != "" {
			if len(v) > 2 && v[0] >= 'A' && v[0] <= 'Z' && v[1] >= 'A' && v[1] <= 'Z' {
				inv.buyerCode, inv.buyerID, inv.buyerVAT = vatToISO(v[:2]), v[2:], true
			} else {
				inv.buyerID = v
			}
			break
		}
	}
	return inv, nil
}

// vatToISO turns an EU VAT prefix into the ISO country code: Greece is EL
// in VAT numbers and GR elsewhere.
func vatToISO(code string) string {
	if code = strings.ToUpper(code); code == "EL" {
		return "GR"
	}
	return code
}

// A finding is something about the archive worth telling: levelWarn may
// change the numbers and status shows it; levelCheck and levelInfo only
// ryczalt check shows, as a warning or a note.
type finding struct {
	level, group, text string
}

const (
	levelWarn  = "warn"
	levelCheck = "check"
	levelInfo  = "info"
)

// checkInvoices finds what a tax office would ask about: invoices from the
// KSeF start with no KSeF number, sent late, or issued after the deadline.
func checkInvoices(invs []invoice) []finding {
	var out []finding
	ksefStart := time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC) // KSeF mandatory for everyone
	for _, inv := range invs {
		if inv.issued.Before(ksefStart) {
			continue
		}
		switch {
		case inv.ksefNumber == "":
			out = append(
				out,
				finding{
					levelCheck,
					"KSeF",
					inv.number + ": no KSeF number in the archive (UPO or .ksef/meta.json)",
				},
			)
		case inv.ksefMode == "":
			out = append(
				out,
				finding{
					levelInfo,
					"KSeF",
					inv.number + ": no UPO kept, the KSeF number comes from .ksef/meta.json",
				},
			)
		case inv.ksefMode == "Offline":
			out = append(
				out,
				finding{
					levelInfo,
					"KSeF",
					inv.number + ": KSeF recorded it as Offline, sent after its issue date",
				},
			)
		}
		if inv.buyerID != "" && !inv.buyerVAT && euVAT[inv.buyerCode] {
			out = append(
				out,
				finding{
					levelCheck,
					"invoicing",
					fmt.Sprintf("%s: the EU buyer is identified by %s, "+
						"not by a VAT number; the revenue register needs a tax id, and EU B2B services the buyer's VAT number",
						inv.number, inv.buyerText()),
				},
			)
		}
		end := inv.serviceEnd
		by := shiftDeadline("PL", time.Date(end.Year(), end.Month()+1, 15, 0, 0, 0, 0, time.UTC))
		if inv.issued.After(by) {
			out = append(
				out,
				finding{
					levelCheck,
					"invoicing",
					fmt.Sprintf(
						"%s: issued %s, after the deadline %s (art. 106i ust. 1 ustawy o VAT)",
						inv.number,
						inv.issued.Format(time.DateOnly),
						by.Format(time.DateOnly),
					),
				},
			)
		}
	}
	return out
}

// printFindings writes one tag per group with its items below.
func printFindings(w *report, fs []finding, levels ...string) (warned bool) {
	var groups []string
	byGroup := map[string][]finding{}
	for _, f := range fs {
		if !slices.Contains(levels, f.level) {
			continue
		}
		if _, ok := byGroup[f.group]; !ok {
			groups = append(groups, f.group)
		}
		byGroup[f.group] = append(byGroup[f.group], f)
	}
	for _, g := range groups {
		warn := slices.ContainsFunc(
			byGroup[g],
			func(f finding) bool { return f.level != levelInfo },
		)
		t := tag(w, cyan, "INFO")
		if warn {
			t, warned = tag(w, yellow, "WARN"), true
		}
		w.println(t, g)
		for _, f := range byGroup[g] {
			w.println("        ", f.text)
		}
	}
	return warned
}

// euVAT are the EU member states, whose businesses carry a VAT number.
var euVAT = map[string]bool{
	"AT": true, "BE": true, "BG": true, "CY": true, "CZ": true, "DE": true, "DK": true,
	"EE": true, "ES": true, "FI": true, "FR": true, "GR": true, "HR": true, "HU": true, "IE": true, "IT": true,
	"LT": true, "LU": true, "LV": true, "MT": true, "NL": true, "PT": true, "RO": true, "SE": true, "SI": true,
	"SK": true,
}
