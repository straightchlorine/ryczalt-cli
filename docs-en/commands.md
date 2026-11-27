---
title: "Commands: init, new, send, paid, year and more"
description: "The ryczalt commands: init, new, add, send, sync, status, paid, void, year, ewid, forms, check, rules. What they print, when they refuse, exit codes, colours."
---

# Commands

Commands with `[-c CONFIG]` read the config file. Without `-c` it is the file in `$RYCZALT_CONFIG`, and when that is unset, `$XDG_CONFIG_HOME/ryczalt.conf` (by default `~/.config/ryczalt.conf`). [Configuration](configuration.md) describes the keys and [Invoice archive](archive.md) the folder layout.

The examples come from an archive with made-up data: seller Jan Kowalski (NIP 1234563218), buyer Beispiel GmbH in Bavaria, one invoice of 4000.00 EUR a month, ryczałt at 12%. In the examples it is 25 November 2026. Paths in the output are shortened to `/home/jan`.

`ryczalt -h`, or `ryczalt` alone, prints the list of commands.

## `ryczalt init [-c CONFIG] [ARCHIVE]`

Creates the config file, readable by its owner only. It does not overwrite an existing file. It writes the file where `-c` says, and without `-c` where the list above says.

`ARCHIVE` is the folder of `YYYY-MM` invoice folders. The default is `~/invoices`. It must be an absolute path or start with `~/`.

The NIP goes into the file if ksef-cli's `ksef.conf` (from `$KSEF_CONFIG`, else `$XDG_CONFIG_HOME/ksef.conf`) has a valid `nip=`. The fields `init` cannot know (`tax.business_start`, `tax.revenue_basis`, `zus.chorobowa`, `client.calendar`) stay empty, and `tax.rate` and `client.payment_days` get `12` and `7`. Commands refuse to work until you fill in the required keys.

The result of `ryczalt init ~/invoices` without a `ksef.conf`:

```
[  OK  ] wrote /home/jan/.config/ryczalt.conf
  fill in, then run ryczalt status:
  - tax.nip
  - tax.business_start
  - tax.revenue_basis
  - zus.chorobowa
  - client.calendar
  - zus.ulga_until and zus.preferential_until, if you had them
```

A second call on the same file:

```
Error: /home/jan/.config/ryczalt.conf already exists, leaving it alone
```

## `ryczalt new [-c CONFIG] [--date DATE] [--number NUMBER] [MONTH]`

Prepares the invoice for a month of work as an `invoice.json` for [Invoify](https://github.com/al1abb/invoify). The file goes into the folder of the issue month (usually the month after the work) and is a copy of the previous invoice's `invoice.json`. You then import it into Invoify, check it and export the PDF.

`MONTH` is the month of work, for example `2026-10`. The default is last month.

- `--date DATE` - the issue date. The default is the first day of the next month, or today if that is later.
- `--number NUMBER` - the invoice number. The default is the previous invoice's number, with the `YYYY-MM` of its issue month replaced by the new invoice's issue month.

From the previous invoice `new` changes the number, the issue date, the due date, the sentence about the NBP rate with the PLN amount in the first item's description, and the sentence about the service period in the payment terms. It copies the rest unchanged. If one of those sentences is missing, it prints a warning. The due date is the issue date plus `client.payment_days`, moved according to `client.calendar`.

The result of `ryczalt new` on 2026-11-25:

```
[  OK  ] wrote /home/jan/invoices/2026-11/invoice.json
         invoice   FV-2026-11-001, issued Wed 2026-11-25, due Wed 2026-12-02
         work      2026-10-01 to 2026-10-31
         rate      4.3955 PLN per EUR, NBP table 212/A/NBP/2026 of 2026-10-30
         revenue   4000.00 EUR = 17 582.00 PLN
         to pay    by Fri 2026-11-20: ryczalt 2 110*, zus 2 757.34, total 4 867.34 PLN
[ INFO ] copied from 2026-10/invoice.json; review the notes in Invoify before exporting
```

It refuses when:

- the issue date falls before the work ends,
- an invoice with that number is already in the archive,
- it cannot derive the number from the previous invoice (give `--number`),
- there is no earlier `invoice.json` to start from,
- the previous invoice is in PLN (`new` only knows foreign-currency invoices so far),
- an `invoice.json` already exists in the target folder,
- the NBP rate is not published yet.

`ryczalt new` a second time:

```
Error: invoice FV-2026-11-001 already exists (2026-11/invoice.json); give another --number
```

## `ryczalt add [-c CONFIG] [--month YYYY-MM] [--move|-m] FILE...`

Puts documents (the invoice PDF from Invoify, a contract, a receipt) into a month folder. By default it is the newest invoice's folder. `--move` or `-m` removes the originals once copied.

Copying keeps the modification time and checks the size. The month folder is created if it does not exist.

The result of `ryczalt add --month 2026-11 contract.pdf`:

```
[  OK  ] copied /home/jan/invoices/2026-11/contract.pdf
```

It refuses when:

- no files are given,
- a file is not a regular file,
- a file of that name already lies in the month folder. It never overwrites, and it checks all the files before it copies the first,
- the archive has no invoice and `--month` is not given,
- `--month` does not look like `2026-12`.

The same command a second time:

```
Error: /home/jan/invoices/2026-11/contract.pdf already exists, leaving it alone
```

## `ryczalt send [-c CONFIG] [MONTH]`

Sends the invoice to KSeF through [ksef-cli](https://codeberg.org/piotrkrzysztof/ksef-cli). It runs `ksef convert --send --sale-date DATE invoice.json ksef-NUMBER.xml`, where `DATE` is the last day of the work. The FA(3) is saved beside it as `ksef-NUMBER.xml` and the UPO as `ksef-NUMBER.upo.xml`. ksef-cli asks before sending.

For an invoice in a foreign currency, `send` passes ksef-cli the NBP rate it counts revenue at (`--rate RATE`) and prints it on stderr. So the FA(3) carries the same rate as the invoice description and the revenue register. A PLN invoice gets no `--rate`.

It sends the one `invoice.json` without a KSeF number that was issued from 1 April 2026 on. `MONTH` names the month of work when several invoices wait. The ksef-cli program is `ksef` on `PATH`, or `bin=` from the `[ksef]` section. The `config=` key is passed to it as `KSEF_CONFIG`.

It warns on stderr when:

- ksef-cli has `env` set to anything but `prod`,
- the invoice is issued on a day other than today. KSeF will then record it as `Offline`. If that is later than the next working day, the program suggests `ryczalt new --date`.

When ksef-cli has saved the UPO and `[r2]` has a `bucket`, `ryczalt sync` runs after the send (without `--delete`). When there is no UPO, because the send was declined at the prompt or KSeF is still processing, nothing more happens. After a declined send, running `ryczalt send` again prepares and offers the invoice again.

The result of `ryczalt send` when ksef-cli's config does not have `env=prod` (the default is `test`). What follows is ksef-cli's own output:

```
[ WARN ] ksef-cli is set to env=test: this goes to the KSeF test environment, not to the real one
[ INFO ] rate 4.3955 PLN per EUR, NBP table 212/A/NBP/2026 of 2026-10-30, as in the revenue register
```

It refuses when:

- `ksef-NUMBER.upo.xml` already lies beside the invoice (the invoice looks sent),
- no invoice waits to be sent,
- several invoices wait and `MONTH` is not given (exit code 2),
- it cannot find ksef-cli.

`ryczalt send` after the invoice has gone:

```
Error: no invoice waiting to be sent (every invoice.json has a KSeF number)
```

## `ryczalt sync [-c CONFIG] [--delete] [--dry-run]`

Pushes the archive to the `[r2]` bucket with `aws s3 sync`, with `profile=` and `endpoint=` from the config. It leaves out Syncthing's files (`.stfolder`, `.stignore`, `.stversions`, `*.sync-conflict-*`). It never pulls. The archive is the original and the bucket the copy.

- `--delete` also removes from the bucket what is gone from the archive.
- `--dry-run` shows what would be copied.

The output comes from `aws`. Without `bucket=` in `[r2]` the command refuses with exit code 2:

```
Error: no [r2] bucket in the config; sync has nowhere to go
```

## `ryczalt status [-c CONFIG] [--year YEAR] [--today DATE]`

Shows the year month by month: revenue from the invoices in PLN, ryczałt, contributions, the payment deadline and what still needs doing. `--year` picks a year other than the one of today's date. `--today` pretends it is another date.

The `Ryczalt*` column is the amount in full, before the paid contributions are deducted. PIT-28 does the deducting. `Social` includes the Labour Fund and the Solidarity Fund (FP/FS).

```
ryczalt 2026
Wed 2026-11-25 | 12% ryczalt | full ZUS with chorobowa | rules official

Month    Invoice         Revenue PLN  Ryczalt*    Social  Health  Due             State
2026-01  FV-2026-02-001    16 852.40     2 022    456.18  498.35  Fri 2026-02-20  [  OK  ]
2026-02  FV-2026-03-001    16 893.20     2 028    456.18  498.35  Fri 2026-03-20  [  OK  ]
2026-03  FV-2026-04-001    17 132.80     2 055    456.18  498.35  Mon 2026-04-20  [  OK  ]
2026-04  FV-2026-05-001    17 014.80     2 042    456.18  830.58  Wed 2026-05-20  [  OK  ]
2026-05  FV-2026-06-001    16 928.80     2 032    456.18  830.58  Mon 2026-06-22  [  OK  ]
2026-06  FV-2026-07-001    17 156.00     2 058    456.18  830.58  Mon 2026-07-20  [  OK  ]
2026-07  FV-2026-08-001    17 238.80     2 069  1 926.76  830.58  Thu 2026-08-20  [  OK  ]
2026-08  FV-2026-09-001    17 331.20     2 080  1 926.76  830.58  Mon 2026-09-21  [  OK  ]
2026-09  FV-2026-10-001    17 491.20     2 099  1 926.76  830.58  Tue 2026-10-20  [  OK  ]
2026-10  FV-2026-11-001    17 582.00     2 110  1 926.76  830.58  Fri 2026-11-20  [ WARN ] late, 2026-11-24
2026-11  -                         -       n/a  1 926.76  830.58  Mon 2026-12-21  in progress

* in full, before deducting paid ZUS (social, half of health); PIT-28 settles the deductions

Year so far   171 621.20 PLN revenue, health bracket 60k-300k

Next payment  Mon 2026-12-21, for 2026-11
  zus             2 757.34  social 1 788.29, health 830.58, FP/FS 138.47
  ryczalt                   not yet invoiced
  total           2 757.34  PLN

1 more note about the archive: ryczalt check
```

The `State` column says where the month stands:

| State | Meaning |
|-------|---------|
| `in progress` | the month is running (`in progress, zus paid` once the contributions are paid) |
| `[  OK  ]` | everything paid on time |
| `[ WARN ] late, DATE` | paid after the deadline, on `DATE` |
| `[ WARN ] due today` | the deadline is today |
| `due in N days` | the deadline is in N days, with `[ WARN ]` at 7 days or fewer (with `zus paid` once the contributions are paid) |
| `due tomorrow` | the deadline is tomorrow, with `[ WARN ]` (with `zus paid` once the contributions are paid) |
| `[ FAIL ] zus, ryczalt` | the deadline has passed and what is named is not paid |
| `[ FAIL ] no invoice; zus unpaid` | no invoice after its deadline and ZUS is not paid |
| `[ FAIL ] no invoice; due by DATE` | no invoice after the invoice deadline: the 15th of the next month, moved off a weekend or holiday (art. 106i ust. 1 of the VAT act) |
| `[ INFO ] not invoiced yet; due by DATE` | the month is over, there is no invoice yet, but the invoice deadline has not passed |
| `suspended` | a suspended month, nothing is due |

Below the table come the year's revenue with the health bracket, the next payment (or the overdue one, as `Overdue`) and notes about the archive. While no payment is recorded, `status` suggests `ryczalt paid --on-due`. The command exits with code 1 when any month shows `[ FAIL ]`.

This is the same year before any payment was recorded and before the October invoice was issued:

```
ryczalt 2026
Wed 2026-11-25 | 12% ryczalt | full ZUS with chorobowa | rules official

Month    Invoice         Revenue PLN  Ryczalt*    Social  Health  Due             State
2026-01  FV-2026-02-001    16 852.40     2 022    456.18  498.35  Fri 2026-02-20  [ FAIL ] zus, ryczalt
2026-02  FV-2026-03-001    16 893.20     2 028    456.18  498.35  Fri 2026-03-20  [ FAIL ] zus, ryczalt
...
2026-10  -                         -       n/a  1 926.76  830.58  Fri 2026-11-20  [ FAIL ] no invoice; zus unpaid
2026-11  -                         -       n/a  1 926.76  830.58  Mon 2026-12-21  in progress

* in full, before deducting paid ZUS (social, half of health); PIT-28 settles the deductions

Year so far   154 039.20 PLN revenue, health bracket 60k-300k

Overdue       Fri 2026-02-20, for 2026-01
  zus               954.53  social 456.18, health 498.35
  ryczalt         2 022.00  in full*
  total           2 976.53  PLN

[ INFO ] no payments recorded yet; if all were paid on time: ryczalt paid --on-due 2026-01..2026-10
```

From 1 January to 20 May the view of the current year also shows the months of last year that still have something open: unpaid ZUS or ryczałt, or a missing invoice. They come before the months of the current year and count towards the next payment and the exit code. The year's revenue and the hint to record payments cover the current year only. The 20 May deadline moves off a weekend or holiday, as in `ryczalt year`.

During the same time a `[ INFO ]` line before the notes lists what is left to settle for last year: PIT-28 (from 15 February to 30 April) and the health settlement (by 20 May). An item drops out after its deadline. The program does not know whether the return was filed or the settlement paid, so the line stays until then. It is absent when last year was suspended throughout.

`status --today 2027-01-10`. The health figure for 2027 is not published yet, hence `n/a` in the `Health` column:

```
ryczalt 2027
Sun 2027-01-10 | 12% ryczalt | full ZUS with chorobowa | rules unknown

Month    Invoice  Revenue PLN  Ryczalt*     Social  Health  Due             State
2026-11  -                  -       n/a   1 926.76  830.58  Mon 2026-12-21  [ FAIL ] no invoice; zus unpaid
2026-12  -                  -       n/a   1 926.76  830.58  Wed 2027-01-20  [ INFO ] not invoiced yet; due by 2027-01-15
2027-01  -                  -       n/a  ~2 052.15     n/a  Mon 2027-02-22  in progress

* in full, before deducting paid ZUS (social, half of health); PIT-28 settles the deductions

Year so far   0.00 PLN revenue, health bracket up to 60k

Overdue       Mon 2026-12-21, for 2026-11
  zus             2 757.34  social 1 788.29, health 830.58, FP/FS 138.47
  ryczalt                   not yet invoiced
  total           2 757.34  PLN

[ INFO ] 2026 still to settle: PIT-28 between 2027-02-15 and 2027-04-30, health settlement by 2027-05-20 (ryczalt year)

1 more note about the archive: ryczalt check
```

## `ryczalt paid [-c CONFIG] [--on DATE | --on-due] PERIOD [KIND [AMOUNT]]`

```
ryczalt paid [-c CONFIG] --list [YEAR]
```

Records a payment for a month (`2026-10`) or a range of months (`2026-01..2026-10`) within one year. Payments go to a `payments.HOST.csv` file in the archive, in the folder of the year paid. The file is append-only.

`KIND` is `zus` (social, health and FP/FS at the amounts due), `social`, `health`, `fpfs` or `ryczalt`. Without `KIND` the program records `zus` and `ryczalt`, each at the amount due. For a month not yet invoiced it records ZUS only.

- By default the payment is dated today. `--on DATE` sets another date, `--on-due` each month's deadline.
- `AMOUNT` records a different amount, or a further payment (`1758`, `830.58` or `830,58`).
- `--list [YEAR]` shows the payments recorded in that year or for its months, with their ids. Without a year it takes the current one.

`ryczalt paid 2025-12 social 456.18 --on 2026-01-20`:

```
[  OK  ] 20260120-59a00d  2025-12 social      456.18 PLN  paid 2026-01-20
[ INFO ] recorded in /home/jan/invoices/2026/payments.laptop.csv
```

`ryczalt paid --on-due 2026-01..2026-03`:

```
[  OK  ] 20260220-48ce0d  2026-01 social      456.18 PLN  paid 2026-02-20
[  OK  ] 20260220-721c4d  2026-01 health      498.35 PLN  paid 2026-02-20
[  OK  ] 20260220-47413c  2026-01 ryczalt   2 022.00 PLN  paid 2026-02-20
[  OK  ] 20260320-979fda  2026-02 social      456.18 PLN  paid 2026-03-20
...
```

When part of it is already recorded, `paid` leaves that as it is and says so. A further payment needs a `KIND` and an `AMOUNT`. `ryczalt paid 2026-10` once that month is recorded:

```
[ INFO ] already recorded, left as is: 2026-10 social, 2026-10 health, 2026-10 fpfs, 2026-10 ryczalt
Error: nothing new to record; a further payment needs a KIND and an AMOUNT
```

`ryczalt paid --list 2026`:

```
Id               Paid on     For      Kind          PLN
20260120-59a00d  2026-01-20  2025-12  social     456.18
20260120-3512ac  2026-01-20  2025-12  health     498.35
20260220-48ce0d  2026-02-20  2026-01  social     456.18
20260220-721c4d  2026-02-20  2026-01  health     498.35
20260220-47413c  2026-02-20  2026-01  ryczalt  2 022.00
...
```

It refuses when:

- `--on` and `--on-due` are given together, or `--on` is in the future,
- `PERIOD` is not a month or a range, or the range does not run forward within one year,
- the payment is dated before the month began,
- the month is not a month of business up to today,
- `AMOUNT` is given for `zus`, without a `KIND`, or for a range, or is not a number like `1758` or `830.58`,
- the amount due is not known yet. Give an `AMOUNT` then,
- everything is already recorded. There is nothing new to record.

`ryczalt paid 2026-11 ryczalt` when November is not invoiced yet:

```
Error: 2026-11 ryczalt: the amount due is not known yet; record it with: ryczalt paid 2026-11 ryczalt AMOUNT
```

## `ryczalt void [-c CONFIG] ID...`

Cancels the payments with the given ids (from `ryczalt paid --list`). It deletes nothing: it appends a cancelling row. It refuses when the payment does not exist or is already cancelled.

`ryczalt void` with an id from the list:

```
[  OK  ] voided 20261124-4590e7 (/home/jan/invoices/2026/payments.laptop.csv)
```

Cancelling the same payment again:

```
Error: no payment 20261124-4590e7 (or it is voided already)
```

## `ryczalt year [-c CONFIG] [--today DATE] [YEAR]`

The year's summary: the PIT-28 figures and the annual health contribution settlement. Until the end of May it takes last year, afterwards this one. It also accepts `--today DATE`. Details and an example in [Year-end](year.md).

## `ryczalt ewid [-c CONFIG] [--csv | --jpk [--correction]] [YEAR]`

The year's revenue register as a table, a CSV file (`--csv`) or a JPK_EWP(4) file (`--jpk`). `--correction` works only with `--jpk`. `--jpk` needs a `[jpk]` section in the config. The default year is as in `year`. Details in [Year-end](year.md).

## `ryczalt forms [-c CONFIG] [--today DATE] [YEAR]`

What goes into PIT-28(28) and block XII of the ZUS DRA for April, field by field. The default year is as in `year`. It also accepts `--today DATE`. Details in [Year-end](year.md).

## `ryczalt check [-c CONFIG]`

Shows everything worth knowing about the archive beyond what `status` shows: drafts that differ from the XML sent, invoices without a KSeF number, invoices issued late, an EU buyer without a VAT number, KSeF session keys left in clear text. It exits with code 1 when any group is a warning and not just information.

```
ryczalt check
/home/jan/invoices | 10 invoices | 36 payments

[ WARN ] invoicing
         FV-2026-11-001: issued 2026-11-25, after the deadline 2026-11-16 (art. 106i ust. 1 ustawy o VAT)
```

## `ryczalt rules [YEAR]`

Prints the figures a year's contributions are computed from, with their source and status (official, forecast, not published yet), and the monthly contributions that follow from them. `YEAR` is four digits starting with `20`, by default the current year. It does not read the config. [How it calculates](rules.md) explains what each figure is for.

```
Rules for 2026, in PLN
  minimum wage                           4 806.00  official  https://www.zus.pl/baza-wiedzy/skladki-wskazniki-odsetki/skladki/wysokosc-skladek-na-ubezpieczenia-spoleczne
  forecast average wage                  9 420.00  official  https://www.zus.pl/baza-wiedzy/skladki-wskazniki-odsetki/skladki/wysokosc-skladek-na-ubezpieczenia-spoleczne
  average wage, Q4 of the year before    9 228.64  official  https://www.biznes.gov.pl/pl/portal/001052

Monthly contributions
  health, yearly revenue up to 60 000      498.35
  health, up to 300 000                    830.58
  health, above 300 000                  1 495.04
  social, preferential                     456.18  420.86 without chorobowa
  social, full                           1 788.29  1 649.82 without chorobowa
  FP/FS, full phase only                   138.47
```

## `ryczalt version`

Prints the program's version. It also works as `ryczalt --version` and `ryczalt -v`.

## Exit codes

| Code | Meaning |
|------|---------|
| 0 | success |
| 1 | an error, or a report with something to fix (`status`, `check`) |
| 2 | wrong use: an unknown command, a bad argument or flag, an error in the config |
| 3 | the config describes a setup that is not supported, e.g. `form=liniowy` |

An unknown command:

```
Error: unknown subcommand "frobnicate" - run ryczalt -h for the list
```

Colours appear only in a terminal. A `NO_COLOR` variable with a non-empty value switches them off, and so does `TERM=dumb`. Redirected output never has colours.
