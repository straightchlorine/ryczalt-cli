---
title: "Jak ryczalt liczy przychód, składki i terminy"
description: "Jak ryczalt liczy miesiąc: data przychodu, kurs NBP, ryczałt narastająco, składki ZUS według etapu i progu, terminy płatności, święta PL i DE, liczby roczne."
---

# Jak liczy

Ta strona opisuje, jak `ryczalt` dochodzi do liczb, które pokazuje `status`: przychodu, ryczałtu, składek i terminów. Opis odpowiada kodowi programu, a przepisy podaje tak, jak cytuje je kod. To nie jest porada podatkowa. Tam, gdzie program zostawia wybór tobie, jest to napisane.

Przykłady liczb pochodzą z archiwum ze zmyślonymi danymi opisanego w [Poleceniach](commands.md): faktura na 4000.00 EUR miesięcznie, ryczałt 12%, składki z chorobowym.

## Data uzyskania przychodu

Od daty zależy, do którego miesiąca i roku trafia przychód i jaki kurs NBP obowiązuje. Wybierasz ją kluczem `tax.revenue_basis` (patrz [Konfiguracja](configuration.md)). Program nie rozstrzyga, który wariant jest właściwy dla twojej umowy. To sprawa do ustalenia z księgowym.

| Wartość | Data przychodu |
|---------|----------------|
| `period_end` | ostatni dzień okresu rozliczeniowego z umowy (art. 14 ust. 1e ustawy o PIT), np. miesięcznego abonamentu |
| `service` | dzień wykonania usługi, chyba że faktura wystawiona jest wcześniej (art. 14 ust. 1c) |

Art. 14 ustawy o PIT stosuje się do ryczałtu przez art. 6 ust. 1 ustawy o zryczałtowanym podatku. Przy `service` program bierze datę wystawienia, gdy wypada przed końcem usługi, a w pozostałych przypadkach koniec usługi. Wcześniejsza wpłata od klienta też mogłaby przesunąć przychód, ale wpłaty od klientów nie są jeszcze zapisywane.

Koniec usługi czyta z faktury FA(3): `P_6_Do`, a gdy go nie ma `P_6`, a gdy i tego nie ma, datę wystawienia `P_1`. Dla szkicu `invoice.json` z Invoify, który nie ma daty usługi, jest to ostatni dzień miesiąca przed datą wystawienia.

## Kurs NBP

Przychód w walucie obcej program przelicza po średnim kursie z tabeli A NBP z ostatniego dnia roboczego poprzedzającego dzień uzyskania przychodu (art. 11a ust. 1 ustawy o PIT). Dzień roboczy to nie weekend i nie polskie święto.

Przykład z archiwum demonstracyjnego. Faktura za sierpień 2026, `period_end`: przychód z 31 sierpnia (poniedziałek), dzień wcześniej to niedziela, więc obowiązuje tabela z piątku 28 sierpnia.

```
8,2026-09-01,2026-08-31,1234563218-20260901-AB12CD34EF56-08,FV-2026-09-001,DE123456789,,,,,17331.20,,,,,17331.20,"EUR 4000.00 x 4.3328, NBP 167/A/NBP/2026 of 2026-08-28"
```

(Wiersz 8 z `ryczalt ewid --csv 2026`; w uwagach jest kwota, kurs, numer tabeli i jej data.)

Szczegóły:

- Program pyta `api.nbp.pl` o zakres dziesięciu dni wstecz i bierze ostatnią tabelę. Wysyła w zapytaniu tylko kod waluty i daty.
- Odpowiedź zapisuje w pamięci podręcznej (`$XDG_CACHE_HOME/ryczalt/nbp`, domyślnie `~/.cache/ryczalt/nbp`).
- Gdy tabela z wymaganego dnia jeszcze się nie ukazała (to dzisiaj przed południem albo dzień z przyszłości), program nie przelicza i podaje komunikat, żeby spróbować później. NBP publikuje tabelę około południa. Starszej tabeli wtedy nie podstawia.
- Faktury w PLN nie są przeliczane.

## Zaokrąglanie

- Przychód w PLN: kwota faktury razy kurs, do groszy, połówki zaokrąglane od zera (tak samo dla korekt).
- Składka: podstawa jest zaokrąglana do groszy, potem osobno składka z każdej stawki, a wyniki się sumuje.
- Podstawa opodatkowania i podatek: do pełnych złotych, połówki w górę (art. 63 § 1 Ordynacji podatkowej).
- Połowa zapłaconej składki zdrowotnej do odliczenia: do grosza, połówki w górę.

## Ryczałt narastająco

Ryczałt liczy się od początku roku (art. 21 ustawy o zryczałtowanym podatku). Za miesiąc to podatek od przychodu narastająco od 1 stycznia, minus podatek policzony wcześniej:

1. przychód narastający zaokrąglany do pełnych złotych,
2. razy stawka z `tax.rate`,
3. wynik do pełnych złotych,
4. minus kwota z poprzedniego miesiąca.

Przykład ze `status`: styczeń 16 852.40, ryczałt 2 022 (12% z 16 852). Luty 33 745.60 narastająco, 12% z 33 746 to 4 049.52, czyli 4 050. Zaliczka za luty to 4 050 minus 2 022, czyli 2 028.

Kolumna `Ryczalt*` w `status` to kwota w całości. Program nie odlicza w niej składek. Odliczenia robi dopiero roczne rozliczenie: składki społeczne wpłacone w roku (bez FP/FS) i połowa wpłaconych składek zdrowotnych (art. 11 ust. 1 i 1a ustawy o zryczałtowanym podatku). Opisuje to [Rozliczenie roku](year.md). Miesiąc bez faktury nie ma ryczałtu, ale ZUS jest należny za każdy miesiąc działalności, poza miesiącami zawieszenia.

Dopuszczalne stawki (`tax.rate`) to 17, 15, 14, 12.5, 12, 10, 8.5, 5.5 i 3 (art. 12 ust. 1 ustawy o zryczałtowanym podatku).

## Składki społeczne według etapu

Etap zależy od miesiąca i kluczy `zus.ulga_until` i `zus.preferential_until` (ostatni miesiąc danego etapu):

| Etap | Kiedy | Składka |
|------|-------|---------|
| ulga na start | do `ulga_until` | brak |
| preferencyjne | do `preferential_until` | podstawa 30% minimalnego wynagrodzenia |
| pełne | później, a gdy kluczy brak, od początku | podstawa 60% prognozowanego przeciętnego wynagrodzenia |

Stawki (art. 22 ustawy o systemie ubezpieczeń społecznych):

| Składka | Stawka | Kiedy |
|---------|--------|-------|
| emerytalna | 19,52% | zawsze |
| rentowa | 8,00% | zawsze |
| wypadkowa | 1,67% | płatnik do 9 ubezpieczonych |
| chorobowa | 2,45% | gdy `zus.chorobowa=yes` |
| FP i FS | 2,45% (1,00% + 1,45%) | tylko w pełnym etapie |

FP i FS są liczone osobno, bo nie odlicza się ich od przychodu. Przykład z roku 2026 (podstawa preferencyjna 1 441.80, pełna 5 652.00, z chorobowym):

| Etap | Społeczna | FP/FS |
|------|-----------|-------|
| preferencyjne | 456.18 | brak |
| pełne | 1 788.29 | 138.47 |

Te same liczby pokazuje `ryczalt rules 2026`, także bez chorobowego.

## Składka zdrowotna

Składka to 9% podstawy. Podstawa zależy od przychodu od początku roku pomniejszonego o składki społeczne, w trzech przedziałach (art. 81 ust. 2h ustawy o świadczeniach opieki zdrowotnej):

| Przychód narastająco | Podstawa miesięczna |
|----------------------|---------------------|
| do 60 000 PLN | 60% przeciętnego wynagrodzenia z IV kwartału poprzedniego roku |
| do 300 000 PLN | 100% |
| powyżej 300 000 PLN | 180% |

W przykładzie składka rośnie w kwietniu z 498.35 do 830.58: przychód narastający 67 893.20 minus składki społeczne 1 824.72 przekracza 60 000. Miesięcznie program zakłada, że składki społeczne zapłacono w terminie. Roczne rozliczenie liczy je z wpłat.

Przedział jest wyznaczany od nowa co miesiąc, a po zakończeniu roku jeden przedział dla całego roku (patrz [Rozliczenie roku](year.md)).

## Terminy płatności

| Co | Termin |
|----|--------|
| ZUS i ryczałt za miesiąc | 20. dzień następnego miesiąca |
| faktura za miesiąc | 15. dzień następnego miesiąca (art. 106i ust. 1 ustawy o VAT) |
| wpis do ewidencji przychodów | 20. dzień miesiąca po uzyskaniu przychodu |
| wpłata klienta | data wystawienia plus `client.payment_days` |

Termin, który wypada w sobotę, niedzielę lub święto, przechodzi na następny dzień roboczy (art. 12 § 5 Ordynacji podatkowej, art. 115 Kodeksu cywilnego, § 193 BGB). Dla ZUS, ryczałtu, faktury i ewidencji święta są polskie. Termin wpłaty klienta przesuwa kalendarz z `client.calendar`, czyli prawo, któremu podlega umowa.

## Kalendarze świąt

`client.calendar` to `PL` albo `DE-<land>` (kod ISO landu bez `DE-`: `BB`, `BE`, `BW`, `BY`, `HB`, `HE`, `HH`, `MV`, `NI`, `NW`, `RP`, `SH`, `SL`, `SN`, `ST`, `TH`), np. `DE-BY` dla Bawarii.

Polskie święta (ustawa o dniach wolnych od pracy), według kodu: 1 i 6 stycznia, Wielkanoc i poniedziałek wielkanocny, 1 i 3 maja, Zielone Świątki, Boże Ciało, 15 sierpnia, 1 i 11 listopada, 24 grudnia (od 2025), 25 i 26 grudnia.

Niemieckie święta ogólne: Neujahr, Karfreitag, Ostermontag, 1 maja, Christi Himmelfahrt, Pfingstmontag, 3 października, 25 i 26 grudnia. Do tego święta tylko niektórych landów:

| Święto | Landy |
|--------|-------|
| Ostersonntag, Pfingstsonntag | BB |
| Fronleichnam | BW, BY, HE, NW, RP, SL |
| Heilige Drei Könige (6 I) | BW, BY, ST |
| Internationaler Frauentag (8 III) | BE (od 2019), MV (od 2023) |
| Mariä Himmelfahrt (15 VIII) | SL |
| Weltkindertag (20 IX) | TH (od 2019) |
| Reformationstag (31 X) | BB, MV, SN, ST, TH oraz HB, HH, NI, SH (od 2018) |
| Allerheiligen (1 XI) | BW, BY, NW, RP, SL |
| Buß- und Bettag | SN (środa 16-22 listopada) |

Święta, które obchodzą tylko niektóre gminy landu, program pomija. Dodajesz je sam jako dni `+MM-DD` po kodzie kalendarza, np. `DE-BY +08-15` dla Monachium albo `DE-BY +08-08` dla Augsburga (Friedensfest). Test sprawdza dzień po dniu święta z kodu względem tabeli w `testdata/holidays.txt` (lata 2025-2030).

## Liczby roczne i ich status

Składki zależą od trzech liczb, które co roku ogłaszają przepisy albo GUS i ZUS. Program ma je wpisane razem ze źródłem i statusem. `ryczalt rules` je pokazuje:

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

| Liczba | Do czego służy |
|--------|----------------|
| minimalne wynagrodzenie | 30% to podstawa składek preferencyjnych |
| prognozowane przeciętne wynagrodzenie | 60% to podstawa składek pełnych |
| przeciętne wynagrodzenie z IV kwartału poprzedniego roku | podstawa składki zdrowotnej ryczałtowca (60%, 100% albo 180%) |

Status liczby:

| Status | Znaczenie | W wyniku |
|--------|-----------|----------|
| `official` | opublikowana w przepisach albo przez GUS lub ZUS | kwota bez znaku |
| `forecast` | z projektu lub prognozy | kwota z `~` |
| `unknown` | jeszcze nieopublikowana | `n/a` |

Kwota wyliczona z kilku liczb ma status najsłabszej z nich. Brakującej liczby program nie bierze z poprzedniego roku: wszystko, co od niej zależy, pokazuje `n/a`. W `status` nagłówek podaje najsłabszy status roku (`rules official`). Tak wygląda rok 2027 w wersji programu, z której pochodzą przykłady. Brakuje w nim jeszcze przeciętnego wynagrodzenia z IV kwartału, a prognozowane jest oznaczone `~`:

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

Każdy rok wymaga aktualizacji w styczniu, gdy GUS i ZUS ogłaszają nowe wartości. Nowa wersja programu dodaje je razem ze źródłem.
