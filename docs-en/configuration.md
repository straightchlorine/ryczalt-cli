---
title: "The ryczalt.conf file: keys and values"
description: "The ryczalt.conf file in key=value format: the tax, zus, client, ksef, r2 and jpk sections. Which keys are required, what values it accepts and what it refuses."
---

# Configuration

The `ryczalt.conf` file uses the `key=value` format, with sections in square
brackets. `ryczalt init` creates the first one.

## Where the file lives

The program looks for it in this order:

1. the `-c FILE` option of the command
2. the `RYCZALT_CONFIG` variable
3. `$XDG_CONFIG_HOME/ryczalt.conf`
4. `~/.config/ryczalt.conf`

Every command except `rules` and `version` accepts `-c`. `init` writes the file
to the same place where the other commands look for it, and never overwrites an
existing one.

If the file is missing or invalid, the command exits with code 2. Code 3 means
a setting the program does not handle (see below).

## Format

- A comment starts with `#` at the beginning of a line, or with ` #` after a
  value.
- A value is read as the shell would read it: the quotes `"…"` and `'…'` are
  optional, and inside quotes you can use spaces and the `#` character.
- `~/` at the start of a path means the home directory.
- Empty lines are skipped.
- An unknown section or key is an error, so a typo does not pass silently.
- A key given twice is an error.
- A required key with an empty value counts as missing.
- The program reports all value errors at once.

The `archive` key stands at the top of the file, before the first section. The
other keys belong in sections.

## Archive

| Key       | Required | Description                                                                           |
|-----------|----------|---------------------------------------------------------------------------------------|
| `archive` | yes      | The folder holding the `YYYY-MM` folders. An absolute path or one starting with `~/`. The folder must exist. |

## The `[tax]` section

| Key                  | Required | Values and notes                                                               |
|----------------------|----------|--------------------------------------------------------------------------------|
| `nip`                | yes      | Your NIP: 10 digits, no dashes, with a valid check digit. Invoices from another seller are not revenue and are skipped. |
| `form`               | yes      | Only `ryczalt`.                                                                |
| `payment`            | yes      | Only `monthly`.                                                                |
| `vat_status`         | yes      | Only `exempt`.                                                                 |
| `rate`               | yes      | The ryczałt rate in percent: `17`, `15`, `14`, `12.5`, `12`, `10`, `8.5`, `5.5` or `3`. |
| `business_start`     | yes      | The first month of the business, `YYYY-MM`.                                    |
| `revenue_basis`      | yes      | `period_end` or `service`. You decide which one, not the program.              |
| `suspended_from`     | no       | The first month the business is suspended in full, `YYYY-MM`. Not before `business_start`. |
| `suspended_until`    | no       | The last month of the suspension. Blank means "until further notice". Needs `suspended_from` and cannot be before it. |

Any non-empty value other than `ryczalt`, `monthly` and `exempt` in `form`, `payment` or
`vat_status` ends the command with code 3.

`period_end` is the last day of the settlement period agreed in the contract
(art. 14 ust. 1e of the PIT act), e.g. for a fixed monthly retainer. `service`
is the day the service is done, unless the invoice or the payment comes earlier
(art. 14 ust. 1c). The calculation is described on the [How it calculates](rules.md)
page.

## The `[zus]` section

| Key                  | Required | Values and notes                                                               |
|----------------------|----------|--------------------------------------------------------------------------------|
| `chorobowa`          | yes      | `yes` if you pay the voluntary sickness insurance, `no` if you pay only the obligatory contributions. |
| `ulga_until`         | no       | The last month of ulga na start, `YYYY-MM`, as PUE ZUS shows it. Blank if you never had it. |
| `preferential_until` | no       | The last month of the preferential contributions, `YYYY-MM`. Must come after `ulga_until`. |

Neither month can be before `business_start`.

## The `[client]` section

| Key            | Required | Values and notes                                                                 |
|----------------|----------|----------------------------------------------------------------------------------|
| `payment_days` | yes      | The number of days to pay, from 0 to 365. `ryczalt new` sets the due date to the issue date plus this many days. |
| `calendar`     | yes      | Whose holidays move the deadline: `PL` or `DE-<state>`.                          |

A deadline that falls on a weekend or holiday moves to the next working day.

For `calendar` the program accepts `PL` and `DE-` followed by one of `BB`, `BE`,
`BW`, `BY`, `HB`, `HE`, `HH`, `MV`, `NI`, `NW`, `RP`, `SH`, `SL`, `SN`, `ST`
or `TH`. After it you can list the days that only the client's town keeps, as
`+MM-DD`, e.g. `DE-BY +08-15`.

## The `[ksef]` section

Both values are optional. They set how `ryczalt send` runs ksef-cli.

| Key      | Default                  | Description                                       |
|----------|--------------------------|---------------------------------------------------|
| `bin`    | `ksef` from `PATH`       | The path to the ksef-cli program.                 |
| `config` | ksef-cli's own file      | Passed to ksef-cli as `KSEF_CONFIG`.              |

## The `[r2]` section

An optional off-site copy of the archive, pushed by `ryczalt sync` and by
`ryczalt send`. It uses the `aws` program.

| Key        | Description                                                                |
|------------|----------------------------------------------------------------------------|
| `bucket`   | An S3 bucket, e.g. on Cloudflare R2. Required when `profile` or `endpoint` is set. |
| `profile`  | The `aws` profile with the credentials.                                    |
| `endpoint` | The service address, e.g. `https://<account>.r2.cloudflarestorage.com`.    |

Without `bucket` no copy is made, and `ryczalt sync` exits with code 2.

## The `[jpk]` section

Only for `ryczalt ewid --jpk`, the JPK_EWP(4) file filed after the year ends.
Without this section the rest of the program works, and `ewid --jpk` reports
the missing keys.

| Key          | Required for `--jpk` | Values and notes                          |
|--------------|----------------------|-------------------------------------------|
| `first_name` | yes                  | First name.                               |
| `surname`    | yes                  | Surname.                                  |
| `birth_date` | yes                  | Date of birth, `YYYY-MM-DD`.              |
| `tax_office` | yes                  | The tax office code, exactly 4 digits.    |
| `email`      | no                   | E-mail address.                           |

## Example

A filled-in file with placeholder data:

```
archive=~/invoices

[tax]
nip=1234563218
form=ryczalt
payment=monthly
vat_status=exempt
rate=12
business_start=2025-01
revenue_basis=period_end

[zus]
ulga_until=2025-06
preferential_until=2027-06
chorobowa=no

[client]
payment_days=7
calendar=DE-BY

[ksef]
bin=ksef
config=~/.config/ksef.conf

[r2]
bucket=my-invoices
profile=r2
endpoint=https://ACCOUNT.r2.cloudflarestorage.com

[jpk]
first_name=Jan
surname=Kowalski
birth_date=1990-05-17
tax_office=1471
```

The full, commented template is `ryczalt.conf.example` in the repository.
`ryczalt init` writes the same text with the archive path filled in, and the NIP when
`ksef.conf` has one.
