---
title: "Ryczałt, ZUS and deadlines from the terminal"
description: "ryczalt reads an invoice archive and shows revenue, ryczałt, ZUS and deadlines by month. It records payments, submits invoices to KSeF and gives PIT-28 figures."
---

# ryczalt-cli

Ryczałt, ZUS and payment deadlines for a sole proprietorship, from the command
line.

`ryczalt` reads an invoice archive (one `YYYY-MM` folder per month) and shows,
month by month, the revenue, the ryczałt, the ZUS contributions and the payment
deadlines.

It records payments, prepares the next invoice and submits it to KSeF through
[ksef-cli](https://codeberg.org/piotrkrzysztof/ksef-cli). At the end of the
year it gives the figures for PIT-28 and for the annual health contribution
settlement.

## Features

- Converts foreign-currency revenue at the NBP rate of the last working day
  before the day the revenue arises (art. 11a of the PIT act)
- Calculates the ryczałt cumulatively from the start of the year, the social
  contributions by phase (ulga na start, preferential, full) and the health
  contribution by bracket
- Moves deadlines that fall on a weekend or holiday, with Polish and German
  holiday calendars
- Records payments (`ryczalt paid`) and shows what is paid and what is overdue
- Prepares next month's invoice as an [Invoify](https://github.com/al1abb/invoify)
  file, copied from the previous one
- Submits the invoice to KSeF through [ksef-cli](https://codeberg.org/piotrkrzysztof/ksef-cli)
  and copies the archive to R2 or another S3 bucket
- Gives the PIT-28 figures and estimates the annual health settlement, also field
  by field for PIT-28 and block XII of the DRA (`ryczalt forms`)
- Handles a suspended business (zawieszenie)
- Warns when the year's revenue nears a health bracket line
- Keeps the revenue register: a table, CSV, or a JPK_EWP(4) file checked
  against the official MF schema
- Checks the archive (`ryczalt check`): missing KSeF numbers, invoices issued
  late, EU buyers identified by something other than a VAT number

## Installation

You need Go 1.26 or newer.

```bash
go install codeberg.org/piotrkrzysztof/ryczalt-cli/cmd/ryczalt@latest
```

The command installs the `ryczalt` program in `~/go/bin`. Prebuilt binaries are
on the [releases page](https://codeberg.org/piotrkrzysztof/ryczalt-cli/releases)
on Codeberg.

Submitting invoices needs [ksef-cli](https://codeberg.org/piotrkrzysztof/ksef-cli)
to be installed. The other commands do not need it.

## Getting started

```bash
ryczalt init ~/invoices    # creates ~/.config/ryczalt.conf (NIP from ksef.conf)
$EDITOR ~/.config/ryczalt.conf
ryczalt status             # this year, month by month
ryczalt year               # PIT-28 and the annual health settlement
```

`init` never overwrites an existing file. The keys are described on the
[Configuration](configuration.md) page, and the folder layout on the
[Invoice archive](archive.md) page.

## Every month

```bash
ryczalt new                         # last month's invoice (invoice.json)
#  Invoify: import, check, export the PDF
ryczalt add -m ~/Downloads/FV.pdf   # the PDF into the month folder
ryczalt send                        # to KSeF through ksef-cli, then a copy to R2
ryczalt paid 2026-11                # once ZUS and ryczałt are paid
```

All commands and their options are on the [Commands](commands.md) page.

## Who is it for?

This version handles one setup: ryczałt od przychodów ewidencjonowanych,
VAT-exempt under art. 113, monthly payments. Other settings in the
configuration file are refused.

The year's contribution figures are built in, each with its source. Every year
needs an update in January, when GUS and ZUS publish the new values.

The program does not decide legal questions. What else it does not do is on the
[About](about.md) page.
