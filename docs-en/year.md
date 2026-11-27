---
title: "Year-end figures: PIT-28, DRA and the register"
description: "What ryczalt gives at year-end: PIT-28(28) and block XII of the ZUS DRA field by field, the revenue register as CSV or JPK_EWP(4), the annual health settlement."
---

# Year-end

`ryczalt` does not file PIT-28, the ZUS DRA or the JPK_EWP file. It works the figures out from the invoices and the recorded payments, and you enter them in the official portals. Every command on this page only reads the archive.

Three commands, `year`, `forms` and `ewid`, take the year from the `YEAR` argument. Without one, until the end of May it is last year, afterwards this one. The commands are described in [Commands](commands.md), and where the figures come from in [How it calculates](rules.md).

The examples come from an archive with made-up data: Jan Kowalski (NIP 1234563218) invoices Beispiel GmbH in Bavaria 4000.00 EUR a month. The business started in June 2025. Ulga na start ran until November 2025, preferential contributions until June 2026, and from July 2026 they are full. In the examples it is 25 November 2026, so the year is still running and the program also works out a year-end estimate.

## PIT-28: `ryczalt year`

```
ryczalt year 2026
Wed 2026-11-25 | 12% ryczalt | year not over: figures so far, and a year-end estimate

Where 2026 stands
  invoiced                  Jan-Oct      10 of 12 months
  ryczalt paid              Jan-Oct      10 of 12 months
  ZUS paid                  Jan-Oct      10 of 12 months + Dec 2025
  next                      Nov          ZUS by Mon 2026-12-21, ryczalt once invoiced
```

The `Where 2026 stands` block shows for how many months of the year there is an invoice, ryczałt paid and ZUS paid. The note `+ Dec 2025` means contributions for a month outside the year (here December), paid in this year. They are deducted in this year. The `next` row gives the nearest deadline.

```
PIT-28 for 2026  filed 2027-02-15 to 2027-04-30
  revenue                      171 621.20  invoiced Jan-Oct
    - social ZUS paid          -10 346.42  for Dec 2025-Oct 2026, paid in 2026
    - 50% of health ZUS paid    -3 903.73  half of 7 807.46: ryczalt deducts half
    = taxable base             157 371.00  rounded to whole PLN
  ryczalt 12%                   18 885.00
    - ryczalt already paid     -20 595.00  Jan-Oct, in full
    = refund so far              1 710.00
  year-end estimate             ~1 975.00  refund if the rest goes like Oct
```

The rows of the settlement, from the top:

| Row | What it is |
|-----|------------|
| `revenue` | revenue from invoices whose revenue date falls in the year, in PLN |
| `- social ZUS paid` | social contributions paid in the year, without FP/FS, for any month |
| `- 50% of health ZUS paid` | half of the health contributions paid in the year |
| `= taxable base` | revenue less the deductions, rounded to whole zloty |
| `ryczalt 12%` | the tax on the base, in whole zloty |
| `- ryczalt already paid` | advances paid for the year's months, December's included |
| `= to pay` or `= refund` | what is left to pay or to get back |

The deductions never exceed the revenue. What is left over is lost. The period in the heading (`filed 2027-02-15 to 2027-04-30`) runs from 15 February to 30 April of the next year, moved off a weekend or holiday. While the year runs, the `year-end estimate` row assumes that every month still to come brings what the last invoiced one did and is paid on its deadline.

## The annual health settlement

```
Estimated health settlement for 2026  due 2027-05-20 with the ZUS DRA for April*
  revenue                     ~206 785.20
    - social ZUS paid         ~-12 134.71  paid in 2026
    = yearly base             ~194 650.00  sets the bracket: 60k-300k
  due for the year               9 966.93  9% of 12 x 9 228.64
    - billed month by month     -8 970.27  Jan-Mar 498.35, Apr-Dec 830.58
    = to pay by 20 May            ~996.66  Jan-Mar billed at 498.35 instead of 830.58

* Nov-Dec assumed like Oct; final once the year is invoiced
```

| Row | What it is |
|-----|------------|
| `revenue` | the year's revenue |
| `- social ZUS paid` | the social contributions paid |
| `= yearly base` | revenue less social, in whole zloty. It picks the bracket: up to 60 000 PLN, up to 300 000 PLN or above |
| `due for the year` | 9% of the number of insured months times the bracket's monthly base |
| `- billed month by month` | the sum of the monthly contributions, in runs of the same amount |
| `= to pay` or `= overpaid` | a top-up by 20 May of the next year (with a note of which months were billed lower) or an overpayment, which you must apply to get back |

The bracket is set by the whole year's revenue, so one month more or less moves every month. In the current year `status` and `year` warn when the year-end estimate lies closer to a line than one month's revenue, on either side. The 20 May date is the deadline of the ZUS DRA for April, which the settlement is filed with.

## The form fields: `ryczalt forms`

`forms` gives the same figures field by field. The PIT-28(28) field numbers come from the MF form definition (crd.e-instytucja.pl, entry 14369), and those of block XII of the ZUS DRA from the Poradnik ZUS DRA of May 2026. They are mapped for the 12% rate only: with another rate `forms` warns that the amounts need care.

```
ryczalt forms 2026
Wed 2026-11-25 | year not over: an estimate*

PIT-28(28)  file 2027-02-15 to 2027-04-30, e-Urzad Skarbowy
  P_4      tax year                                    2026
  P_30     revenue, own business, 12%           ~206 785.20  part D, row 1
  P_57     revenue together, 12%                ~206 785.20
  P_63     revenue together                     ~206 785.20
  P_98     social contributions paid             ~12 134.71  paid in 2026
  P_105    health contributions (art. 11 1a)      ~4 319.02  50% of 8 638.04 paid
  P_137    revenue after deductions, 12%        ~190 331.47
  P_202    tax base, 12%                        ~190 331.00  whole PLN
  P_217    ryczalt 12%                           ~22 840.00
  P_232    ryczalt due                           ~22 840.00  whole PLN
  P_236    ryczalt paid                          ~24 815.00  advances for the year's months
  P_238    overpayment                            ~1 975.00  refunded after filing
  P_239-50 ryczalt per month (part Q)                        Jan 2 022, Feb 2 028, Mar 2 055, Apr 2 042, May 2 032, Jun 2 058, Jul 2 069, Aug 2 080, Sep 2 099, Oct 2 110, Nov 2 110, Dec 2 110
  P_100, P_104, P_107, P_114, P_125, P_177 and P_224-P_230 follow from these; the rest is 0
```

PIT-28(28):

| Field | What to enter |
|-------|---------------|
| `P_4` | the tax year |
| `P_30` | revenue from the own business, 12% (part D, row 1) |
| `P_57`, `P_63` | the same revenue, together |
| `P_98` | social contributions paid |
| `P_105` | half of the health contributions paid (art. 11 ust. 1a of the ryczałt act) |
| `P_137` | revenue after deductions |
| `P_202` | the tax base, whole zloty |
| `P_217`, `P_232` | ryczałt 12%, due |
| `P_236` | ryczałt paid, that is the advances for the year's months |
| `P_237` or `P_238` | to pay or overpayment |
| `P_239-50` | the ryczałt for each month, as paid (part Q) |

The fields `P_100`, `P_104`, `P_107`, `P_114`, `P_125`, `P_177` and `P_224` to `P_230` follow from those above, and the program takes the rest as 0.

```
ZUS DRA for April, block XII  health settlement, by 2027-05-20
  01       settlement year                             2026
  21       taxation form: ryczalt                         X
  22       revenue                              ~194 650.49  less social ZUS paid
  23       annual base                          ~110 743.68  12 x 9 228.64
  24       annual contribution                    ~9 966.93  9% of field 23
  25       monthly contributions, sum             ~8 970.27  as on the monthly DRAs
  26       to pay                                   ~996.66
  28       to pay, together                         ~996.66  also into block VI and IX.02; one transfer to your NRS account

* the rest of the year assumed like the last invoiced month; run it again once the year is invoiced
```

Block XII of the ZUS DRA for April:

| Field | What to enter |
|-------|---------------|
| `01` | the settlement year |
| `21` | the form of taxation: ryczałt (`X`) |
| `22` | revenue less the social contributions paid |
| `23` | the annual base: the number of months times the bracket's monthly base |
| `24` | the annual contribution: 9% of field 23 |
| `25` | the sum of the monthly contributions, as on the monthly DRAs |
| `26` or `27` | to pay or overpaid |
| `28` or `29` | the same, together. A top-up also goes into blocks VI and IX.02, in one transfer to your NRS account. A refund of an overpayment needs an application (RZS-R) |

Field `24` is 9% of the annual base, not the sum of twelve monthly amounts, so it can differ by a few grosze. While the year is not over, every amount carries `~` and is an estimate: the rest of the year is worked out like the last invoiced month. Run `forms` again once the year is invoiced.

The amounts in the example are an estimate as of 25 November. For a year that is already invoiced, `year` and `forms` print the state `the year is complete` and amounts without `~`.

## The revenue register: `ryczalt ewid`

The year's revenue register (regulation of the Minister of Finance of 6 September 2025, Dz.U. 2025 poz. 1294). Each revenue is one row, in the order of the revenue dates. The entry date is the day the invoice was issued, or the revenue date when that is later.

```
ryczalt ewid 2026
ewidencja przychodow, 10 entries; --csv for a spreadsheet, --jpk for JPK_EWP(4)

Lp  Entered      Revenue     Document        KSeF number                          Buyer           12% PLN
 1  2026-02-01   2026-01-31  FV-2026-02-001  -                                    DE123456789   16 852.40
 2  2026-03-01   2026-02-28  FV-2026-03-001  -                                    DE123456789   16 893.20
 3  2026-04-01   2026-03-31  FV-2026-04-001  1234563218-20260401-AB12CD34EF56-03  DE123456789   17 132.80
 4  2026-05-01   2026-04-30  FV-2026-05-001  1234563218-20260501-AB12CD34EF56-04  DE123456789   17 014.80
 5  2026-06-01   2026-05-31  FV-2026-06-001  1234563218-20260601-AB12CD34EF56-05  DE123456789   16 928.80
 6  2026-07-01   2026-06-30  FV-2026-07-001  1234563218-20260701-AB12CD34EF56-06  DE123456789   17 156.00
 7  2026-08-01   2026-07-31  FV-2026-08-001  1234563218-20260801-AB12CD34EF56-07  DE123456789   17 238.80
 8  2026-09-01   2026-08-31  FV-2026-09-001  1234563218-20260901-AB12CD34EF56-08  DE123456789   17 331.20
 9  2026-10-01   2026-09-30  FV-2026-10-001  1234563218-20261001-AB12CD34EF56-09  DE123456789   17 491.20
10  2026-11-25*  2026-10-31  FV-2026-11-001  1234563218-20261125-AB12CD34EF56-0A  DE123456789   17 582.00
                                                                                  total        171 621.20

*: entered after the 20th of the next month (par. 6)
```

In a terminal with colours, a yellow cell marks an entry made late, that is after the 20th of the next month (§ 6 of the regulation), or a buyer without a VAT number. A legend follows the table. In the example the last invoice was issued on 25 November for October, so its entry is late. Without colours (piped output, or `NO_COLOR` set) those cells end in `*` and the legend starts with `*`, as in the example.

With `--csv` the command prints the register's 17 columns, headed as the regulation names them: the rate columns `17%` to `3%`, `Ogółem` and `Uwagi`. The amount goes into the column of the rate from the config. `Uwagi` holds the currency conversion (the amount, the rate, the NBP table and its date).

```
Lp.,Data wpisu,Data uzyskania przychodu,Numer KSeF,Numer dowodu,Identyfikator podatkowy kontrahenta,17%,15%,14%,12.5%,12%,10%,8.5%,5.5%,3%,Ogółem,Uwagi
1,2026-02-01,2026-01-31,,FV-2026-02-001,DE123456789,,,,,16852.40,,,,,16852.40,"EUR 4000.00 x 4.2131, NBP 020/A/NBP/2026 of 2026-01-30"
2,2026-03-01,2026-02-28,,FV-2026-03-001,DE123456789,,,,,16893.20,,,,,16893.20,"EUR 4000.00 x 4.2233, NBP 040/A/NBP/2026 of 2026-02-27"
...
```

With `--jpk` it prints a JPK_EWP(4) file. It needs a `[jpk]` section in the config with the keys `first_name`, `surname`, `birth_date` (`YYYY-MM-DD`) and `tax_office` (the four-digit tax office code). The `email` key is optional. Without the required keys the command exits with code 2 and names the missing ones. `--correction` changes the purpose of the filing from a first file (`CelZlozenia` 1) to a correction (2). The program's tests check the file against the MF schema when `xmllint` is installed.

The result goes to standard output, so you save it by redirecting. Warnings go to stderr, never into the file:

```bash
ryczalt ewid --jpk 2026 > JPK_EWP.xml
```

```
<?xml version="1.0" encoding="UTF-8"?>
<JPK xmlns="http://jpk.mf.gov.pl/wzor/2024/10/30/10301/" xmlns:etd="http://crd.gov.pl/xml/schematy/dziedzinowe/mf/2022/01/05/eD/DefinicjeTypy/">
  <Naglowek>
    <KodFormularza kodSystemowy="JPK_EWP (4)" wersjaSchemy="1-0">JPK_EWP</KodFormularza>
    <WariantFormularza>4</WariantFormularza>
    <CelZlozenia>1</CelZlozenia>
    <DataWytworzeniaJPK>2026-11-25T21:49:56Z</DataWytworzeniaJPK>
    <DataOd>2026-01-01</DataOd>
    <DataDo>2026-12-31</DataDo>
    <KodUrzedu>1471</KodUrzedu>
  </Naglowek>
  <Podmiot1 rola="Podatnik">
    <OsobaFizyczna>
      <etd:NIP>1234563218</etd:NIP>
      <etd:ImiePierwsze>Jan</etd:ImiePierwsze>
      <etd:Nazwisko>Kowalski</etd:Nazwisko>
      <etd:DataUrodzenia>1990-01-01</etd:DataUrodzenia>
    </OsobaFizyczna>
  </Podmiot1>
  <EWPWiersz>
    <K_1>1</K_1>
    <K_2>2026-02-01</K_2>
    <K_3>2026-01-31</K_3>
    <K_4>FV-2026-02-001</K_4>
    <K_6>DE</K_6>
    <K_7>123456789</K_7>
    <K_8>16852.40</K_8>
    <K_9>12</K_9>
    <K_10>EUR 4000.00 x 4.2131, NBP 020/A/NBP/2026 of 2026-01-30</K_10>
  </EWPWiersz>
...
```

The JPK_EWP(4) file goes in with PIT-28 (that is how `ryczalt -h` describes it). The program does not send it.

## A suspended business

The keys `tax.suspended_from` and `tax.suspended_until` (the first and the last month of the suspension, and without `suspended_until` until further notice) change the figures like this:

- suspended months carry no contributions: no social, no health, no FP/FS,
- they do not count as insured months, so the health settlement multiplies the base by a smaller number of months,
- a missing invoice for such a month is not a lapse, and `status` shows `suspended`,
- a year in which the suspension runs to the end is treated as over once all the months before the suspension are invoiced,
- an invoice with revenue in a suspended month is counted, but `status` and `year` warn about it.

A suspension counts in whole months. Below, the same year with `suspended_from=2026-11`:

```
ryczalt year 2026
Wed 2026-11-25 | 12% ryczalt | business suspended since 2026-11

Where 2026 stands
  invoiced                  Jan-Oct      10 of 10 months
  ryczalt paid              Jan-Oct      10 of 10 months
  ZUS paid                  Jan-Oct      10 of 10 months + Dec 2025

PIT-28 for 2026  filed 2027-02-15 to 2027-04-30
  revenue                      171 621.20  invoiced Jan-Oct
    - social ZUS paid          -10 346.42  for Dec 2025-Oct 2026, paid in 2026
    - 50% of health ZUS paid    -3 903.73  half of 7 807.46: ryczalt deducts half
    = taxable base             157 371.00  rounded to whole PLN
  ryczalt 12%                   18 885.00
    - ryczalt already paid     -20 595.00  Jan-Oct, in full
    = refund                     1 710.00

Health settlement for 2026  due 2027-05-20 with the ZUS DRA for April
  revenue                      171 621.20
    - social ZUS paid          -10 346.42  paid in 2026
    = yearly base              161 275.00  sets the bracket: 60k-300k
  due for the year               8 305.78  9% of 10 x 9 228.64
    - billed month by month     -7 309.11  Jan-Mar 498.35, Apr-Oct 830.58, Nov 0.00
    = to pay by 20 May             996.67  Jan-Mar billed at 498.35 instead of 830.58, Nov billed at 0.00 instead of 830.58
```

The months with invoices are now 10 of 10, and the annual contribution is 9% of 10 bases, not 12. November enters the contribution line as `Nov 0.00`.

```
ZUS DRA for April, block XII  health settlement, by 2027-05-20
  01       settlement year                             2026
  21       taxation form: ryczalt                         X
  22       revenue                               161 274.78  less social ZUS paid
  23       annual base                            92 286.40  10 x 9 228.64
  24       annual contribution                     8 305.78  9% of field 23
  25       monthly contributions, sum              7 309.11  as on the monthly DRAs
  26       to pay                                    996.67
  28       to pay, together                          996.67  also into block VI and IX.02; one transfer to your NRS account
```

## How it falls in time

The deadlines the program itself gives:

| What | Deadline |
|------|----------|
| the invoice for a month | the 15th of the next month (art. 106i ust. 1 of the VAT act) |
| ZUS and ryczałt for a month | the 20th of the next month |
| an entry in the revenue register | the 20th of the month after the revenue (§ 6 of the regulation) |
| PIT-28 | from 15 February to 30 April of the next year |
| the health settlement | 20 May of the next year, with the ZUS DRA for April |

Each of them moves off a Saturday, a Sunday or a holiday to the next working day. [How it calculates](rules.md) describes the moving. The program gives no deadline for filing the JPK_EWP file.
