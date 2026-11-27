---
title: "Plik ryczalt.conf: klucze i wartości"
description: "Plik ryczalt.conf w formacie klucz=wartość: sekcje tax, zus, client, ksef, r2 i jpk. Które klucze są wymagane, jakie wartości program przyjmuje i co odrzuca."
---

# Konfiguracja

Plik `ryczalt.conf` w formacie `klucz=wartość`, z sekcjami w nawiasach
kwadratowych. Pierwszy plik tworzy `ryczalt init`.

## Gdzie leży plik

Program szuka go w tej kolejności:

1. opcja `-c PLIK` polecenia
2. zmienna `RYCZALT_CONFIG`
3. `$XDG_CONFIG_HOME/ryczalt.conf`
4. `~/.config/ryczalt.conf`

Opcję `-c` przyjmują wszystkie polecenia poza `rules` i `version`. `init`
zapisuje plik w tym samym miejscu, w którym inne polecenia go szukają, i nigdy
nie nadpisuje istniejącego.

Gdy pliku nie ma albo jest błędny, polecenie kończy się kodem 2. Kod 3 oznacza
ustawienie, którego program nie obsługuje (patrz niżej).

## Format

- Komentarz zaczyna się od `#` na początku wiersza albo od ` #` po wartości.
- Wartość czyta się jak w powłoce: cudzysłowy `"…"` i `'…'` są opcjonalne,
  a w cudzysłowie można użyć spacji i znaku `#`.
- `~/` na początku ścieżki oznacza katalog domowy.
- Puste wiersze są pomijane.
- Nieznana sekcja albo nieznany klucz to błąd, więc literówka nie przejdzie
  bez wiadomości.
- Klucz podany dwa razy w tej samej sekcji to błąd.
- Klucz wymagany z pustą wartością liczy się jak brakujący.
- Błędy wartości program zgłasza wszystkie naraz.

Klucz `archive` stoi na początku pliku, przed pierwszą sekcją. Pozostałe leżą
w sekcjach.

## Archiwum

| Klucz     | Wymagany | Opis                                                                                  |
|-----------|----------|---------------------------------------------------------------------------------------|
| `archive` | tak      | Folder z folderami `RRRR-MM`. Ścieżka bezwzględna albo zaczynająca się od `~/`. Folder musi istnieć. |

## Sekcja `[tax]`

| Klucz                | Wymagany | Wartości i uwagi                                                                 |
|----------------------|----------|----------------------------------------------------------------------------------|
| `nip`                | tak      | NIP, 10 cyfr bez kresek, z poprawną cyfrą kontrolną. Faktury innego wystawcy nie są przychodem i są pomijane. |
| `form`               | tak      | Tylko `ryczalt`.                                                                 |
| `payment`            | tak      | Tylko `monthly`.                                                                 |
| `vat_status`         | tak      | Tylko `exempt`.                                                                  |
| `rate`               | tak      | Stawka ryczałtu w procentach: `17`, `15`, `14`, `12.5`, `12`, `10`, `8.5`, `5.5` lub `3`. |
| `business_start`     | tak      | Pierwszy miesiąc działalności, `RRRR-MM`.                                        |
| `revenue_basis`      | tak      | `period_end` albo `service`. Którą wybrać, rozstrzyga użytkownik, nie program.   |
| `suspended_from`     | nie      | Pierwszy miesiąc w pełni zawieszonej działalności, `RRRR-MM`. Nie wcześniej niż `business_start`. |
| `suspended_until`    | nie      | Ostatni miesiąc zawieszenia. Puste oznacza „do odwołania". Wymaga `suspended_from` i nie może być przed nim. |

Niepusta wartość inna niż `ryczalt`, `monthly` i `exempt` w `form`, `payment` lub
`vat_status` kończy polecenie kodem 3.

`period_end` to ostatni dzień okresu rozliczeniowego z umowy (art. 14 ust. 1e
ustawy o PIT), np. przy stałej miesięcznej opłacie. `service` to dzień
wykonania usługi, chyba że faktura albo płatność wypadły wcześniej (art. 14
ust. 1c). Szczegóły liczenia opisuje strona [Jak liczy](rules.md).

## Sekcja `[zus]`

| Klucz                | Wymagany | Wartości i uwagi                                                                 |
|----------------------|----------|----------------------------------------------------------------------------------|
| `chorobowa`          | tak      | `yes`, jeśli opłacasz dobrowolne ubezpieczenie chorobowe, `no`, jeśli tylko obowiązkowe składki. |
| `ulga_until`         | nie      | Ostatni miesiąc ulgi na start, `RRRR-MM`, tak jak pokazuje PUE ZUS. Puste, jeśli jej nie było. |
| `preferential_until` | nie      | Ostatni miesiąc preferencyjnych składek, `RRRR-MM`. Musi być po `ulga_until`.     |

Żaden z miesięcy nie może być przed `business_start`.

## Sekcja `[client]`

| Klucz          | Wymagany | Wartości i uwagi                                                                   |
|----------------|----------|------------------------------------------------------------------------------------|
| `payment_days` | tak      | Liczba dni na zapłatę, od 0 do 365. `ryczalt new` liczy termin jako datę wystawienia plus tyle dni. |
| `calendar`     | tak      | Czyje święta przesuwają termin: `PL` albo `DE-<kraj związkowy>`.                   |

Termin wypadający w weekend lub święto przesuwa się na następny dzień roboczy.

Dla `calendar` program przyjmuje `PL` oraz `DE-` z jednym ze skrótów: `BB`,
`BE`, `BW`, `BY`, `HB`, `HE`, `HH`, `MV`, `NI`, `NW`, `RP`, `SH`, `SL`, `SN`,
`ST`, `TH`. Po nim mogą stać dni, które świętuje tylko miasto kontrahenta,
w formacie `+MM-DD`, np. `DE-BY +08-15`.

## Sekcja `[ksef]`

Obie wartości są opcjonalne. Określają, jak `ryczalt send` uruchamia ksef-cli.

| Klucz    | Domyślnie                | Opis                                              |
|----------|--------------------------|---------------------------------------------------|
| `bin`    | `ksef` z `PATH`          | Ścieżka do programu ksef-cli.                     |
| `config` | własny plik ksef-cli     | Przekazywany ksef-cli jako `KSEF_CONFIG`.         |

## Sekcja `[r2]`

Opcjonalna kopia archiwum, którą wypycha `ryczalt sync` oraz `ryczalt send`.
Korzysta z programu `aws`.

| Klucz      | Opis                                                                     |
|------------|--------------------------------------------------------------------------|
| `bucket`   | Bucket S3, np. na Cloudflare R2. Wymagany, gdy podano `profile` lub `endpoint`. |
| `profile`  | Profil `aws` z danymi dostępowymi.                                       |
| `endpoint` | Adres usługi, np. `https://<konto>.r2.cloudflarestorage.com`.            |

Bez `bucket` kopia nie powstaje, a `ryczalt sync` kończy się kodem 2.

## Sekcja `[jpk]`

Tylko dla `ryczalt ewid --jpk`, czyli pliku JPK_EWP(4) składanego po zakończeniu
roku. Bez tej sekcji reszta programu działa, a `ewid --jpk` zgłasza brakujące
klucze.

| Klucz        | Wymagany dla `--jpk` | Wartości i uwagi                              |
|--------------|----------------------|-----------------------------------------------|
| `first_name` | tak                  | Imię.                                         |
| `surname`    | tak                  | Nazwisko.                                     |
| `birth_date` | tak                  | Data urodzenia, `RRRR-MM-DD`.                 |
| `tax_office` | tak                  | Kod urzędu skarbowego, dokładnie 4 cyfry.     |
| `email`      | nie                  | Adres e-mail.                                 |

## Przykład

Wypełniony plik z przykładowymi danymi:

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

Pełny, skomentowany wzór to `ryczalt.conf.example` w repozytorium. `ryczalt
init` zapisuje ten sam tekst ze ścieżką archiwum oraz z NIP-em, jeśli ma go `ksef.conf`.
