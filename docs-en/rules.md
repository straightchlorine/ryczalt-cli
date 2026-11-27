---
title: "How ryczalt calculates revenue and deadlines"
description: "How ryczalt works out a month: revenue date, NBP rate, cumulative ryczałt, ZUS by phase and bracket, payment deadlines, PL and DE holidays, yearly figures."
---

# How it calculates

This page describes how `ryczalt` arrives at the numbers `status` shows: revenue, ryczałt, contributions and deadlines. The description matches the program's code, and it gives the legal provisions as the code cites them. It is not tax advice. Where the program leaves a choice to you, the page says so.

The example figures come from the archive with made-up data described in [Commands](commands.md): an invoice of 4000.00 EUR a month, ryczałt at 12%, contributions with the voluntary sickness insurance.

## The revenue date

The date decides which month and year the revenue falls into and which NBP rate applies. You choose it with the key `tax.revenue_basis` (see [Configuration](configuration.md)). The program does not decide which variant is right for your contract. That is for you to settle with your accountant.

| Value | The revenue date |
|-------|------------------|
| `period_end` | the last day of the settlement period agreed in the contract (art. 14 ust. 1e of the PIT act), e.g. a monthly retainer |
| `service` | the day the service is done, unless the invoice is issued earlier (art. 14 ust. 1c) |

Art. 14 of the PIT act applies to ryczałt through art. 6 ust. 1 of the ryczałt act (ustawa o zryczałtowanym podatku). Under `service` the program takes the issue date when it falls before the end of the service, and otherwise the end of the service. An earlier payment from the client could also move the revenue, but payments from clients are not recorded yet.

It reads the end of the service from the FA(3) invoice: `P_6_Do`, and when that is missing `P_6`, and when that is missing as well the issue date `P_1`. For an Invoify `invoice.json` draft, which has no service date, it is the last day of the month before the issue date.

## The NBP rate

The program converts revenue in a foreign currency at the mid rate from NBP's table A of the last business day before the revenue date (art. 11a ust. 1 of the PIT act). A business day is not a weekend and not a Polish holiday.

An example from the demo archive. The invoice for August 2026, `period_end`: the revenue date is 31 August (a Monday), the day before is a Sunday, so the table of Friday 28 August applies.

```
8,2026-09-01,2026-08-31,1234563218-20260901-AB12CD34EF56-08,FV-2026-09-001,DE123456789,,,,,17331.20,,,,,17331.20,"EUR 4000.00 x 4.3328, NBP 167/A/NBP/2026 of 2026-08-28"
```

(Row 8 of `ryczalt ewid --csv 2026`; the notes hold the amount, the rate, the table number and its date.)

The details:

- The program asks `api.nbp.pl` for a range of ten days back and takes the last table. The request carries only the currency code and the dates.
- It stores the answer in a cache (`$XDG_CACHE_HOME/ryczalt/nbp`, by default `~/.cache/ryczalt/nbp`).
- When the table of the required day has not come out yet (it is today before noon, or a day in the future), the program does not convert and prints a message to try again later. NBP publishes the table around noon. It does not substitute an older table then.
- Invoices in PLN are not converted.

## Rounding

- Revenue in PLN: the invoice amount times the rate, to grosze, halves away from zero (the same for corrections).
- A contribution: the base is rounded to grosze, then the contribution of each rate separately, and the results are summed.
- The tax base and the tax: to whole zloty, halves up (art. 63 § 1 of the Tax Ordinance).
- Half of the health contribution paid, for the deduction: to the grosz, halves up.

## Ryczałt cumulatively

Ryczałt is worked out from 1 January (art. 21 of the ryczałt act). For a month it is the tax on the revenue cumulative from 1 January, less the tax worked out earlier:

1. the cumulative revenue rounded to whole zloty,
2. times the rate from `tax.rate`,
3. the result to whole zloty,
4. less the amount of the previous month.

An example from `status`: January 16 852.40, ryczałt 2 022 (12% of 16 852). February cumulative 33 745.60, 12% of 33 746 is 4 049.52, which is 4 050. The advance for February is 4 050 less 2 022, which is 2 028.

The `Ryczalt*` column in `status` is the amount in full. The program does not deduct contributions in it. The deductions come only in the annual settlement: the social contributions paid in the year (without FP/FS) and half of the health contributions paid (art. 11 ust. 1 and 1a of the ryczałt act). [Year-end](year.md) describes that. A month without an invoice has no ryczałt, but ZUS is due for every month of the business except the suspended ones.

The permitted rates (`tax.rate`) are 17, 15, 14, 12.5, 12, 10, 8.5, 5.5 and 3 (art. 12 ust. 1 of the ryczałt act).

## Social contributions by phase

The phase depends on the month and the keys `zus.ulga_until` and `zus.preferential_until` (the last month of that phase):

| Phase | When | Contribution |
|-------|------|--------------|
| ulga na start | up to `ulga_until` | none |
| preferential | up to `preferential_until` | a base of 30% of the minimum wage |
| full | after that, and from the start when the keys are empty | a base of 60% of the forecast average wage |

The rates (art. 22 of the social insurance act):

| Contribution | Rate | When |
|--------------|------|------|
| pension (emerytalna) | 19.52% | always |
| disability (rentowa) | 8.00% | always |
| accident (wypadkowa) | 1.67% | payers with up to 9 insured |
| sickness (chorobowa) | 2.45% | when `zus.chorobowa=yes` |
| FP and FS | 2.45% (1.00% + 1.45%) | the full phase only |

FP and FS are worked out apart because they cannot be deducted from revenue. An example for 2026 (the preferential base is 1 441.80, the full one 5 652.00, with sickness insurance):

| Phase | Social | FP/FS |
|-------|--------|-------|
| preferential | 456.18 | none |
| full | 1 788.29 | 138.47 |

`ryczalt rules 2026` shows the same figures, also without sickness insurance.

## The health contribution

The contribution is 9% of the base. The base depends on the revenue from the start of the year less the social contributions, in three brackets (art. 81 ust. 2h of the health insurance act):

| Cumulative revenue | Monthly base |
|--------------------|--------------|
| up to 60 000 PLN | 60% of the average wage of Q4 of the year before |
| up to 300 000 PLN | 100% |
| above 300 000 PLN | 180% |

In the example the contribution rises in April from 498.35 to 830.58: the cumulative revenue of 67 893.20 less social contributions of 1 824.72 exceeds 60 000. Month by month the program assumes the social contributions were paid on time. The annual settlement works them out from the payments.

The bracket is set anew every month, and after the year ends there is one bracket for the whole year (see [Year-end](year.md)).

## Payment deadlines

| What | Deadline |
|------|----------|
| ZUS and ryczałt for a month | the 20th of the next month |
| the invoice for a month | the 15th of the next month (art. 106i ust. 1 of the VAT act) |
| an entry in the revenue register | the 20th of the month after the revenue |
| the client's payment | the issue date plus `client.payment_days` |

A deadline that falls on a Saturday, a Sunday or a holiday moves to the next working day (art. 12 § 5 of the Tax Ordinance, art. 115 of the Civil Code, § 193 BGB). For ZUS, ryczałt, the invoice and the register the holidays are Polish. The client's payment deadline is moved by the calendar from `client.calendar`, that is by the law the contract is under.

## Holiday calendars

`client.calendar` is `PL` or `DE-<state>` (the ISO code of the state without `DE-`: `BB`, `BE`, `BW`, `BY`, `HB`, `HE`, `HH`, `MV`, `NI`, `NW`, `RP`, `SH`, `SL`, `SN`, `ST`, `TH`), e.g. `DE-BY` for Bavaria.

The Polish holidays (the act on days free from work), as the code has them: 1 and 6 January, Easter Sunday and Easter Monday, 1 and 3 May, Pentecost, Corpus Christi, 15 August, 1 and 11 November, 24 December (from 2025), 25 and 26 December.

The German holidays in every state: Neujahr, Karfreitag, Ostermontag, 1 May, Christi Himmelfahrt, Pfingstmontag, 3 October, 25 and 26 December. On top of those come holidays of only some states:

| Holiday | States |
|---------|--------|
| Ostersonntag, Pfingstsonntag | BB |
| Fronleichnam | BW, BY, HE, NW, RP, SL |
| Heilige Drei Könige (6 Jan) | BW, BY, ST |
| Internationaler Frauentag (8 Mar) | BE (from 2019), MV (from 2023) |
| Mariä Himmelfahrt (15 Aug) | SL |
| Weltkindertag (20 Sep) | TH (from 2019) |
| Reformationstag (31 Oct) | BB, MV, SN, ST, TH, and HB, HH, NI, SH (from 2018) |
| Allerheiligen (1 Nov) | BW, BY, NW, RP, SL |
| Buß- und Bettag | SN (the Wednesday between 16 and 22 November) |

Holidays that only some municipalities of a state keep, the program leaves out. You add them yourself as `+MM-DD` days after the calendar code, e.g. `DE-BY +08-15` for Munich or `DE-BY +08-08` for Augsburg (Friedensfest). A test checks the program's holidays, day by day, against the table in `testdata/holidays.txt` (the years 2025-2030).

## The yearly figures and their status

The contributions rest on three figures that are published every year in law or by GUS and ZUS. The program has them built in, each with its source and status. `ryczalt rules` shows them:

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

| Figure | What it is for |
|--------|----------------|
| the minimum wage | 30% of it is the base of preferential contributions |
| the forecast average wage | 60% of it is the base of full contributions |
| the average wage of Q4 of the year before | the base of the health contribution under ryczałt (60%, 100% or 180%) |

A figure's status:

| Status | Meaning | In the output |
|--------|---------|---------------|
| `official` | published in law or by GUS or ZUS | the amount without a mark |
| `forecast` | from a draft or a projection | the amount with `~` |
| `unknown` | not published yet | `n/a` |

An amount worked out from several figures takes the status of the weakest. The program does not take a missing figure from the previous year: everything that depends on it shows `n/a`. In `status` the header gives the year's weakest status (`rules official`). This is what 2027 looks like in the version of the program the examples come from. It still lacks the Q4 average wage, and the forecast one is marked `~`:

```
Rules for 2027, in PLN
  minimum wage                           4 950.00  official  Dz.U. 2026 poz. 1213
  forecast average wage                ~10 033.00  forecast  draft 2027 budget
  average wage, Q4 of the year before         n/a  unknown

Monthly contributions
  health, yearly revenue up to 60 000         n/a
  health, up to 300 000                       n/a
  health, above 300 000                       n/a
  social, preferential                     469.85  433.47 without chorobowa
  social, full                          ~1 904.66  ~1 757.17 without chorobowa
  FP/FS, full phase only                  ~147.49
[ WARN ] no 2027 figure for average wage, Q4 of the year before; what depends on it shows n/a
[ INFO ] ~ marks an amount based on a forecast, not the official figure
```

Every year needs an update in January, when GUS and ZUS publish the new values. A new version of the program adds them with their source.
