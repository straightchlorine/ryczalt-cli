---
title: "Invoice archive: folder layout and files"
description: "How to lay out the invoice archive for ryczalt: a YYYY-MM folder per month, the invoice.json, FA(3) and UPO files it reads, what it writes and never overwrites."
---

# Invoice archive

The archive is a plain folder on disk, set by the `archive` key in the
[configuration](configuration.md). The program reads invoices from it and
writes a few files into it. There is no database.

## Folder layout

Each month has its own folder named `YYYY-MM`, e.g. `2026-11`. The program
skips folders with other names. A month folder may be a symbolic link to
another place.

`ryczalt new` writes the invoice into the folder of the issue month, and
`ryczalt add` by default puts files into the issue-month folder of the newest
invoice. When reading the archive, the program takes the dates from the
invoice itself, not from the folder name.

```
~/invoices/
|-- 2026-10/
|   |-- invoice.json
|   |-- ksef-FV-2026-10.xml
|   |-- ksef-FV-2026-10.upo.xml
|   `-- FV-2026-10.pdf
|-- 2026-11/
|   `-- invoice.json
`-- 2026/
    `-- payments.laptop.csv
```

A folder with a year only, such as `2026`, holds the payments of that year.

## What the program reads

In a month folder it reads only `*.xml` files, `invoice.json` and
`.ksef/meta.json`. Everything else (PDFs, contracts, receipts) is none of its
business. Files larger than 10 MB are skipped.

| File                    | Where from and what the program takes |
|-------------------------|---------------------------------------|
| `invoice.json`          | An Invoify export, directly in the month folder only. The number, issue date, currency and amount. The buyer's VAT number from the receiver's custom field named `VAT`, `NrVatUE`, `VAT ID` or `NIP`. The end of the service is the last day of the month before the issue date. |
| `*.xml` with a `Faktura` element | An FA(3) invoice, in the month folder or in a subfolder. The number (`P_2`), issue date (`P_1`), end of the service (`P_6_Do`, then `P_6`, then `P_1`), currency, amount (`P_15`), the seller's NIP and the buyer's identifier. |
| `*.xml` with a `Potwierdzenie` element | A UPO from KSeF. The invoice number, the KSeF number and the sending mode (`Online` or `Offline`). |
| `.ksef/meta.json`       | An older layout: the invoice number and the KSeF number. When there is also a UPO, the UPO wins. |

### What follows from this

- A KSeF number from a UPO or from `.ksef/meta.json` means the invoice went to
  KSeF. `ryczalt send` submits only `invoice.json` drafts without such a
  number. `ryczalt check` reports an invoice without a number if it was issued
  on or after 1 April 2026.
- When the same invoice exists as `invoice.json` and as FA(3), the program
  calculates from the XML. If the amount, currency or period differ,
  `ryczalt status` warns. A different issue date on the draft alone is only a
  note in `ryczalt check`.
- The same invoice in two files of the same kind (two `invoice.json` or two XML) with identical data counts once. If the data
  differ, the program stops with an error.
- An invoice issued by a NIP other than `tax.nip` is not revenue and is skipped,
  e.g. a purchase invoice pulled from KSeF.
- An FA(3) invoice of a kind other than plain (`RodzajFaktury` not `VAT`) or
  with a VAT amount above zero stops the program with an error. Corrections and
  advance invoices are not supported yet.
- An XML file the program cannot read is skipped with a warning.
- A KSeF session key stored in clear text in `.ksef/meta.json` (`aes_key_hex`)
  is reported by `ryczalt check`.

## What the program writes

| Command         | What is created                                                      |
|-----------------|----------------------------------------------------------------------|
| `ryczalt new`   | `YYYY-MM/invoice.json` in the issue-month folder: a copy of the previous invoice with the new number, dates, NBP rate and PLN amount. It creates the folder if missing. |
| `ryczalt add`   | Copies of the given files in the month folder (`--month` or the default). With `--move` the program removes the originals afterwards. |
| `ryczalt send`  | Runs `ksef convert --send` on `invoice.json`. Next to it ksef-cli writes `ksef-NUMBER.xml` and, once sent, `ksef-NUMBER.upo.xml`. In `NUMBER`, characters other than `A-Z`, `a-z`, `0-9`, `.`, `_` and `-` become `-`. |
| `ryczalt paid`, `ryczalt void` | Rows in `YYYY/payments.HOSTNAME.csv`, where `YYYY` is the year of the payment date (for `void`, the year it is cancelled). |
| `ryczalt sync`  | Nothing in the archive. It pushes the archive to the `[r2]` bucket, skipping Syncthing files (`.stfolder`, `.stignore`, `.stversions`, conflict copies). |

`ryczalt init` writes only the configuration file, nothing in the archive. The
program keeps NBP rates outside the archive, in `$XDG_CACHE_HOME/ryczalt` or
`~/.cache/ryczalt`.

### The payments file

A `payments.HOSTNAME.csv` file has a header and one row per payment:

```
id,paid_on,period,kind,amount,note
20261105-a1b2c3,2026-11-05,2026-10,ryczalt,1000.00,
```

- `period` is the month the payment is for.
- `kind` is `social`, `health`, `fpfs` or `ryczalt`. A `void` row cancels the
  payment whose id is in the last column.
- Each computer writes to its own file, so that a sync tool (e.g. Syncthing)
  never sees two machines writing the same file.
- A file with a wrong header or row is skipped as a whole, with a warning. A
  `payments.*` file with another name, such as a sync conflict copy, is skipped
  and the program asks you to merge it by hand.

## What the program never overwrites

- `ryczalt new`, `ryczalt add` and `ryczalt init` refuse when the target file
  already exists.
- `ryczalt add` checks all the files before copying, so an error does not leave
  the work half done. It keeps the modification time.
- `ryczalt send` refuses when `ksef-NUMBER.upo.xml` already exists.
- Payments are never edited or deleted. A mistake is cancelled by
  `ryczalt void`, which appends a row.
- `ryczalt sync` never pulls anything from R2. It deletes there only with
  `--delete`.

The commands are described on the [Commands](commands.md) page.
