<h1 align="center">ryczalt-cli</h1>

<div align="center">

[Polski](README.md) &middot; [Codeberg](https://codeberg.org/piotrkrzysztof/ryczalt-cli) &middot; [GitHub](https://github.com/straightchlorine/ryczalt-cli)

Ryczałt, ZUS and payment deadlines for Polish sole traders, from the command line.

</div>

<br>

`ryczalt` reads an invoice archive (one `YYYY-MM` folder per month) and shows,
month by month, revenue, ryczałt, ZUS contributions and payment deadlines. It
records payments, prepares the next invoice, sends it to KSeF through
[ksef-cli](https://codeberg.org/piotrkrzysztof/ksef-cli), and at the end of the
year gives the figures for PIT-28 and the annual health contribution settlement.

## Features

- Converts foreign-currency revenue at the NBP rate from the last business day
  before the revenue date (art. 11a of the PIT act)
- Works out ryczałt cumulatively from 1 January, social contributions by phase
  (ulga na start, preferential, full) and the health contribution by bracket
- Moves deadlines that fall on a weekend or holiday, with Polish and German
  holiday calendars
- Records payments (`ryczalt paid`) and shows what is paid and what is overdue
- Prepares next month's invoice as an [Invoify](https://github.com/al1abb/invoify)
  file, copied from the previous one
- Sends the invoice to KSeF through [ksef-cli](https://codeberg.org/piotrkrzysztof/ksef-cli) and copies
  the archive to R2 or another S3 bucket
- Gives the PIT-28 figures and estimates the annual health settlement
- Warns when the year's revenue nears a health bracket line

## Getting started

Go 1.26 or newer is required.

```bash
go install codeberg.org/piotrkrzysztof/ryczalt-cli/cmd/ryczalt@latest

ryczalt init ~/invoices    # writes ~/.config/ryczalt.conf (NIP from ksef.conf)
$EDITOR ~/.config/ryczalt.conf
ryczalt status             # this year, month by month
ryczalt year               # PIT-28 and the annual health settlement
```

Every month:

```bash
ryczalt new                         # last month's invoice (invoice.json)
#  Invoify: import, check, export the PDF
ryczalt add -m ~/Downloads/INV.pdf  # the PDF into the month folder
ryczalt send                        # to KSeF through ksef-cli, then a copy to R2
ryczalt paid 2026-11                # once ZUS and ryczałt are paid
```

## Who is it for?

This version handles one setup: ryczałt od przychodów ewidencjonowanych,
VAT-exempt under art. 113, monthly payments.

The year's contribution figures are built in, each with its source. Every year
needs an update in January, when GUS and ZUS publish the new values.

---

<div align="center">

Bogu samemu - cześć i chwała na wieki wieków.

</div>
