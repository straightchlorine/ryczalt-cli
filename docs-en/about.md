---
title: "About ryczalt-cli, its limits and license"
description: "What ryczalt-cli does not do, how the code is laid out and how to run the tests, including the JPK_EWP check with xmllint. MIT license, author and repository."
---

# About

## What the tool does not do

- It handles one setup: ryczałt od przychodów ewidencjonowanych, VAT-exempt
  under art. 113, monthly payments. It refuses other settings in `tax.form`,
  `tax.payment` and `tax.vat_status` when it loads the configuration.
- It knows one ryczałt rate at a time, set in `tax.rate`.
- It files nothing. PIT-28, the JPK_EWP file and the DRA are prepared from its
  figures and filed by you through the official portals. It submits only
  invoices to KSeF, through ksef-cli.
- It makes no transfers. `ryczalt paid` only records a payment.
- It does not create the invoice PDF. Invoify does that, and `ryczalt new`
  prepares the `invoice.json` for it.
- `ryczalt new` works only for foreign-currency invoices. It copies the
  previous invoice, so you prepare the first one yourself.
- It does not support correction or advance invoices. Such an invoice in the
  archive stops the program with an error.
- It is not tax advice. The program does not decide legal questions, e.g. the
  choice of `revenue_basis` or of the rate. The contribution figures must be
  updated every year, and each value has its source in the program
  (`ryczalt rules`).

## Development

All code lives in `cmd/ryczalt/` as a single `main` package and uses only the
Go standard library.

```bash
go test ./...
```

The JPK_EWP test checks the file against the official MF schema from
`testdata/` with `xmllint`. Without `xmllint` that part of the test is skipped.

## License

MIT License. See [LICENSE](https://codeberg.org/piotrkrzysztof/ryczalt-cli/src/branch/main/LICENSE) for details.

## Author

Piotr Krzysztof Lis, [piotrkrzysztof.dev](https://piotrkrzysztof.dev). The source code is on [Codeberg](https://codeberg.org/piotrkrzysztof/ryczalt-cli), with a mirror on [GitHub](https://github.com/straightchlorine/ryczalt-cli).
