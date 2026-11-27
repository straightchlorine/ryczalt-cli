---
title: "Polecenia init, new, send, paid, year i inne"
description: "Polecenia ryczalt: init, new, add, send, sync, status, paid, void, year, ewid, forms, check, rules. Co wypisują, kiedy odmawiają, kody wyjścia i kolory."
---

# Polecenia

Polecenia z `[-c CONFIG]` czytają plik konfiguracyjny. Bez `-c` jest to plik z `$RYCZALT_CONFIG`, a gdy zmiennej brak, `$XDG_CONFIG_HOME/ryczalt.conf` (domyślnie `~/.config/ryczalt.conf`). Klucze opisuje [Konfiguracja](configuration.md), układ folderów [Archiwum faktur](archive.md).

Przykłady pochodzą z archiwum ze zmyślonymi danymi: sprzedawca Jan Kowalski (NIP 1234563218), nabywca Beispiel GmbH z Bawarii, jedna faktura na 4000.00 EUR miesięcznie, ryczałt 12%. W przykładach jest 25 listopada 2026. Ścieżki w wynikach skrócono do `/home/jan`.

`ryczalt -h` albo samo `ryczalt` wypisuje listę poleceń.

## `ryczalt init [-c CONFIG] [ARCHIVE]`

Tworzy plik konfiguracyjny z uprawnieniami tylko dla właściciela. Nie nadpisuje istniejącego pliku. Zapisuje go w miejscu z `-c`, a bez `-c` w miejscu, które podaje lista powyżej.

`ARCHIVE` to folder z folderami faktur `RRRR-MM`. Domyślnie `~/invoices`. Musi być ścieżką bezwzględną albo zaczynać się od `~/`.

NIP trafia do pliku, jeśli `ksef.conf` programu ksef-cli (z `$KSEF_CONFIG`, a bez niej z `$XDG_CONFIG_HOME/ksef.conf`) ma poprawny `nip=`. Pola, których `init` nie zna (`tax.business_start`, `tax.revenue_basis`, `zus.chorobowa`, `client.calendar`), zostają puste, a `tax.rate` i `client.payment_days` dostają wartości `12` i `7`. Polecenia odmawiają pracy, dopóki nie wypełnisz wymaganych kluczy.

Wynik `ryczalt init ~/invoices` bez `ksef.conf`:

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

Drugie wywołanie na tym samym pliku:

```
Error: /home/jan/.config/ryczalt.conf already exists, leaving it alone
```

## `ryczalt new [-c CONFIG] [--date DATE] [--number NUMBER] [MONTH]`

Przygotowuje fakturę za miesiąc pracy jako plik `invoice.json` dla [Invoify](https://github.com/al1abb/invoify). Plik trafia do folderu miesiąca wystawienia (zwykle następnego po miesiącu pracy) i jest kopią poprzedniej faktury z `invoice.json`. Dalej importujesz go w Invoify, sprawdzasz i eksportujesz PDF.

`MONTH` to miesiąc pracy, na przykład `2026-10`. Domyślnie poprzedni miesiąc.

- `--date DATE` - data wystawienia. Domyślnie pierwszy dzień następnego miesiąca albo dzisiaj, jeśli to później.
- `--number NUMBER` - numer faktury. Domyślnie numer poprzedniej faktury, w którym `RRRR-MM` miesiąca jej wystawienia zastąpiono miesiącem wystawienia nowej.

Z poprzedniej faktury `new` zmienia numer, datę wystawienia, termin płatności, zdanie o kursie NBP z kwotą w PLN w opisie pierwszej pozycji i zdanie o okresie usługi w warunkach płatności. Resztę kopiuje bez zmian. Gdy któregoś z tych zdań brakuje, wypisuje ostrzeżenie. Termin płatności to data wystawienia plus `client.payment_days`, przesunięta według `client.calendar`.

Wynik `ryczalt new` w dniu 2026-11-25:

```
[  OK  ] wrote /home/jan/invoices/2026-11/invoice.json
         invoice   FV-2026-11-001, issued Wed 2026-11-25, due Wed 2026-12-02
         work      2026-10-01 to 2026-10-31
         rate      4.3955 PLN per EUR, NBP table 212/A/NBP/2026 of 2026-10-30
         revenue   4000.00 EUR = 17 582.00 PLN
         to pay    by Fri 2026-11-20: ryczalt 2 110*, zus 2 757.34, total 4 867.34 PLN
[ INFO ] copied from 2026-10/invoice.json; review the notes in Invoify before exporting
```

Odmawia, gdy:

- data wystawienia wypada przed końcem miesiąca pracy,
- faktura o tym numerze już jest w archiwum,
- nie da się wyprowadzić numeru z poprzedniej faktury (podaj `--number`),
- nie ma wcześniejszego `invoice.json`, z którego można zacząć,
- poprzednia faktura jest w PLN (`new` zna na razie tylko faktury w walucie obcej),
- `invoice.json` w folderze docelowym już istnieje,
- kurs NBP nie jest jeszcze opublikowany.

`ryczalt new` po raz drugi:

```
Error: invoice FV-2026-11-001 already exists (2026-11/invoice.json); give another --number
```

## `ryczalt add [-c CONFIG] [--month YYYY-MM] [--move|-m] FILE...`

Wkłada dokumenty (PDF faktury z Invoify, umowę, potwierdzenie) do folderu miesiąca. Domyślnie do folderu najnowszej faktury. `--move` albo `-m` usuwa oryginały po skopiowaniu.

Kopiowanie zachowuje czas modyfikacji i sprawdza rozmiar. Folder miesiąca powstaje, jeśli go nie ma.

Wynik `ryczalt add --month 2026-11 contract.pdf`:

```
[  OK  ] copied /home/jan/invoices/2026-11/contract.pdf
```

Odmawia, gdy:

- nie podano plików,
- plik nie jest zwykłym plikiem,
- plik o tej nazwie już leży w folderze miesiąca. Nic nie nadpisuje, a sprawdza wszystkie pliki, zanim skopiuje pierwszy,
- w archiwum nie ma faktury, a nie podano `--month`,
- `--month` nie wygląda jak `2026-12`.

To samo polecenie po raz drugi:

```
Error: /home/jan/invoices/2026-11/contract.pdf already exists, leaving it alone
```

## `ryczalt send [-c CONFIG] [MONTH]`

Wysyła fakturę do KSeF przez [ksef-cli](https://codeberg.org/piotrkrzysztof/ksef-cli). Wywołuje `ksef convert --send --sale-date DATA invoice.json ksef-NUMER.xml`, gdzie `DATA` to ostatni dzień pracy. FA(3) zapisuje się obok jako `ksef-NUMER.xml`, a UPO jako `ksef-NUMER.upo.xml`. Przed wysłaniem pyta ksef-cli.

Przy fakturze w walucie obcej `send` przekazuje ksef-cli kurs NBP, po którym liczy przychód (`--rate KURS`), i wypisuje go na stderr. Dzięki temu FA(3) ma ten sam kurs co opis faktury i ewidencja przychodów. Faktura w PLN nie dostaje `--rate`.

Wysyła ten jedyny `invoice.json` bez numeru KSeF, wystawiony od 1 kwietnia 2026. `MONTH` wskazuje miesiąc pracy, gdy czeka kilka faktur. Program ksef-cli to `ksef` z `PATH` albo `bin=` z sekcji `[ksef]`. Klucz `config=` trafia do niego jako `KSEF_CONFIG`.

Ostrzega na stderr, gdy:

- ksef-cli ma ustawione `env` inne niż `prod`,
- faktura jest wystawiona innego dnia niż dzisiaj. KSeF zapisze ją wtedy jako `Offline`. Gdy to później niż następny dzień roboczy, program podpowiada `ryczalt new --date`.

Gdy ksef-cli zapisał UPO i w `[r2]` jest `bucket`, po wysyłce działa `ryczalt sync` (bez `--delete`). Gdy UPO nie ma, bo wysyłkę odrzucono przy pytaniu albo KSeF jeszcze przetwarza, nic dalej się nie dzieje. Po odmowie kolejne `ryczalt send` przygotowuje i proponuje fakturę od nowa.

Wynik `ryczalt send`, gdy ksef-cli nie ma `env=prod` w konfiguracji (domyślnie jest `test`). Dalej wypisuje już sam ksef-cli:

```
[ WARN ] ksef-cli is set to env=test: this goes to the KSeF test environment, not to the real one
[ INFO ] rate 4.3955 PLN per EUR, NBP table 212/A/NBP/2026 of 2026-10-30, as in the revenue register
```

Odmawia, gdy:

- obok faktury jest już `ksef-NUMER.upo.xml` (faktura wygląda na wysłaną),
- żadna faktura nie czeka na wysłanie,
- czeka kilka faktur, a nie podano `MONTH` (kod wyjścia 2),
- nie znajduje ksef-cli.

`ryczalt send` po wysyłce:

```
Error: no invoice waiting to be sent (every invoice.json has a KSeF number)
```

## `ryczalt sync [-c CONFIG] [--delete] [--dry-run]`

Wysyła archiwum do kubełka z `[r2]` poleceniem `aws s3 sync`, z `profile=` i `endpoint=` z konfiguracji. Pomija pliki Syncthing (`.stfolder`, `.stignore`, `.stversions`, `*.sync-conflict-*`). Nigdy nie pobiera. Archiwum jest oryginałem, a kubełek kopią.

- `--delete` usuwa też z kubełka to, czego brakuje w archiwum.
- `--dry-run` pokazuje, co zostałoby skopiowane.

Wynik pochodzi z `aws`. Bez `bucket=` w `[r2]` polecenie odmawia z kodem 2:

```
Error: no [r2] bucket in the config; sync has nowhere to go
```

## `ryczalt status [-c CONFIG] [--year YEAR] [--today DATE]`

Pokazuje rok miesiąc po miesiącu: przychód z faktur w PLN, ryczałt, składki, termin płatności i to, co jeszcze zostaje do zrobienia. `--year` wybiera inny rok niż ten z dzisiejszej daty. `--today` udaje inną datę.

Kolumna `Ryczalt*` to kwota w całości, przed odliczeniem opłaconych składek. Odlicza dopiero PIT-28. `Social` zawiera też Fundusz Pracy i Solidarnościowy (FP/FS).

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

Kolumna `State` mówi, jak stoi miesiąc:

| Stan | Znaczenie |
|------|-----------|
| `in progress` | miesiąc trwa (`in progress, zus paid`, gdy składki opłacono) |
| `[  OK  ]` | wszystko opłacone na czas |
| `[ WARN ] late, DATA` | opłacone po terminie, dnia `DATA` |
| `[ WARN ] due today` | termin mija dzisiaj |
| `due in N days` | termin za N dni, z `[ WARN ]` przy 7 dniach lub mniej (z dopiskiem `zus paid`, gdy składki opłacono) |
| `due tomorrow` | termin jest jutro, z `[ WARN ]` (z dopiskiem `zus paid`, gdy składki opłacono) |
| `[ FAIL ] zus, ryczalt` | termin minął, a wymienione nie jest opłacone |
| `[ FAIL ] no invoice; zus unpaid` | brak faktury po terminie, a ZUS nie jest opłacony |
| `[ FAIL ] no invoice; due by DATA` | brak faktury po terminie jej wystawienia, czyli po 15. dniu następnego miesiąca, przesuniętym z weekendu lub święta (art. 106i ust. 1 ustawy o VAT) |
| `[ INFO ] not invoiced yet; due by DATA` | miesiąc minął, faktury jeszcze nie ma, ale termin faktury nie minął |
| `suspended` | miesiąc zawieszenia, nic nie jest należne |

Pod tabelą jest przychód od początku roku z progiem składki zdrowotnej, następna płatność (albo zaległa, jako `Overdue`) i uwagi o archiwum. Dopóki nie zapisano żadnej wpłaty, `status` podpowiada `ryczalt paid --on-due`. Polecenie kończy się kodem 1, gdy któryś miesiąc ma `[ FAIL ]`.

Tak wygląda ten sam rok, zanim zapisano wpłaty i wystawiono fakturę za październik:

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

Od 1 stycznia do 20 maja widok bieżącego roku pokazuje też miesiące poprzedniego roku, w których coś jeszcze czeka: nieopłacone ZUS lub ryczałt albo brak faktury. Są przed miesiącami bieżącego roku i liczą się do następnej płatności oraz do kodu wyjścia. Przychód od początku roku i podpowiedź o zapisaniu wpłat obejmują tylko bieżący rok. Termin 20 maja przesuwa się z weekendu lub święta, jak w `ryczalt year`.

W tym samym czasie przed uwagami jest linia `[ INFO ]` z tym, co zostało do rozliczenia za poprzedni rok: PIT-28 (od 15 lutego do 30 kwietnia) i rozliczenie składki zdrowotnej (do 20 maja). Pozycja znika po swoim terminie. Program nie wie, czy zeznanie złożono ani czy rozliczenie opłacono, więc do tego czasu linia zostaje. Nie ma jej, gdy poprzedni rok był w całości zawieszony.

`status --today 2027-01-10`. Kwota składki zdrowotnej na 2027 nie jest jeszcze opublikowana, stąd `n/a` w kolumnie `Health`:

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

Zapisuje wpłatę za miesiąc (`2026-10`) albo za zakres miesięcy (`2026-01..2026-10`) w jednym roku. Wpłaty trafiają do pliku `payments.HOST.csv` w folderze roku wpłaty, w archiwum. Plik tylko się dopisuje.

`KIND` to `zus` (składka społeczna, zdrowotna i FP/FS w należnych kwotach), `social`, `health`, `fpfs` albo `ryczalt`. Bez `KIND` program zapisuje `zus` i `ryczalt`, każde w należnej kwocie. Dla miesiąca jeszcze niezafakturowanego zapisuje tylko ZUS.

- Domyślnie wpłata jest z dzisiaj. `--on DATE` ustawia inną datę, `--on-due` termin każdego miesiąca.
- `AMOUNT` zapisuje inną kwotę albo kolejną wpłatę (`1758`, `830.58` albo `830,58`).
- `--list [YEAR]` pokazuje wpłaty zapisane w danym roku albo za jego miesiące, wraz z identyfikatorami. Bez roku bierze bieżący.

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

Gdy część jest już zapisana, `paid` zostawia ją i mówi o tym. Kolejną wpłatę zapiszesz tylko z `KIND` i `AMOUNT`:

`ryczalt paid 2026-10` po zapisaniu tego miesiąca:

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

Odmawia, gdy:

- `--on` i `--on-due` podano razem, albo `--on` jest w przyszłości,
- `PERIOD` nie jest miesiącem ani zakresem, albo zakres nie biegnie do przodu w jednym roku,
- wpłata jest sprzed początku miesiąca,
- miesiąc nie jest miesiącem działalności do dziś,
- `AMOUNT` podano dla `zus`, bez `KIND` albo dla zakresu, albo nie jest liczbą jak `1758` czy `830.58`,
- należna kwota nie jest jeszcze znana. Podaj wtedy `AMOUNT`,
- wszystko jest już zapisane. Nic nowego do zapisania.

`ryczalt paid 2026-11 ryczalt`, gdy listopad nie jest jeszcze zafakturowany:

```
Error: 2026-11 ryczalt: the amount due is not known yet; record it with: ryczalt paid 2026-11 ryczalt AMOUNT
```

## `ryczalt void [-c CONFIG] ID...`

Anuluje wpłaty o podanych identyfikatorach (z `ryczalt paid --list`). Nic nie usuwa: dopisuje wiersz anulujący. Odmawia, gdy wpłaty nie ma albo jest już anulowana.

`ryczalt void` z identyfikatorem z listy:

```
[  OK  ] voided 20261124-4590e7 (/home/jan/invoices/2026/payments.laptop.csv)
```

Ponowne anulowanie tej samej wpłaty:

```
Error: no payment 20261124-4590e7 (or it is voided already)
```

## `ryczalt year [-c CONFIG] [--today DATE] [YEAR]`

Podsumowanie roku: kwoty do PIT-28 i roczne rozliczenie składki zdrowotnej. Do końca maja bierze rok poprzedni, potem bieżący. Przyjmuje też `--today DATE`. Szczegóły i przykład w [Rozliczeniu roku](year.md).

## `ryczalt ewid [-c CONFIG] [--csv | --jpk [--correction]] [YEAR]`

Ewidencja przychodów roku jako tabela, plik CSV (`--csv`) albo plik JPK_EWP(4) (`--jpk`). `--correction` działa tylko z `--jpk`. `--jpk` wymaga w konfiguracji sekcji `[jpk]`. Domyślny rok jak w `year`. Szczegóły w [Rozliczeniu roku](year.md).

## `ryczalt forms [-c CONFIG] [--today DATE] [YEAR]`

Co wpisać w PIT-28(28) i w bloku XII ZUS DRA za kwiecień, pole po polu. Domyślny rok jak w `year`. Przyjmuje też `--today DATE`. Szczegóły w [Rozliczeniu roku](year.md).

## `ryczalt check [-c CONFIG]`

Pokazuje wszystko, co warto wiedzieć o archiwum, poza tym, co widać w `status`: szkice różniące się od wysłanego XML, faktury bez numeru KSeF, faktury wystawione po terminie, nabywcę z UE bez numeru VAT, zapisane wprost klucze sesji KSeF. Kończy się kodem 1, gdy któraś z grup jest ostrzeżeniem, a nie samą informacją.

```
ryczalt check
/home/jan/invoices | 10 invoices | 36 payments

[ WARN ] invoicing
         FV-2026-11-001: issued 2026-11-25, after the deadline 2026-11-16 (art. 106i ust. 1 ustawy o VAT)
```

## `ryczalt rules [YEAR]`

Wypisuje liczby, z których program liczy składki w danym roku, ze źródłem i statusem (oficjalna, prognoza, jeszcze niepublikowana), oraz wynikające z nich miesięczne składki. `YEAR` to cztery cyfry zaczynające się od `20`, domyślnie bieżący rok. Nie czyta konfiguracji. Znaczenie każdej liczby opisuje [Jak liczy](rules.md).

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

Wypisuje wersję programu. Działa też jako `ryczalt --version` i `ryczalt -v`.

## Kody wyjścia

| Kod | Znaczenie |
|-----|-----------|
| 0 | powodzenie |
| 1 | błąd, albo raport z czymś do poprawy (`status`, `check`) |
| 2 | błędne użycie: nieznane polecenie, zły argument lub flaga, błąd w konfiguracji |
| 3 | konfiguracja opisuje nieobsługiwany układ, np. `form=liniowy` |

Nieznane polecenie:

```
Error: unknown subcommand "frobnicate" - run ryczalt -h for the list
```

Kolory pojawiają się tylko w terminalu. Wyłącza je zmienna `NO_COLOR` o niepustej wartości albo `TERM=dumb`. Przekierowane wyjście jest zawsze bez kolorów.
