---
title: "Rozliczenie roku: PIT-28, DRA i ewidencja"
description: "Co daje ryczalt przy rozliczeniu roku: kwoty do PIT-28(28) i bloku XII ZUS DRA, ewidencja przychodów jako CSV lub JPK_EWP(4), rozliczenie składki zdrowotnej."
---

# Rozliczenie roku

`ryczalt` nie składa PIT-28, ZUS DRA ani pliku JPK_EWP. Liczy kwoty z faktur i zapisanych wpłat, a ty wpisujesz je w oficjalnych portalach. Wszystkie polecenia z tej strony tylko czytają archiwum.

Trzy polecenia, `year`, `forms` i `ewid`, biorą rok z argumentu `YEAR`. Bez argumentu do końca maja jest to rok poprzedni, potem bieżący. Opisy poleceń są w [Poleceniach](commands.md), a skąd biorą się liczby, w [Jak liczy](rules.md).

Przykłady pochodzą z archiwum ze zmyślonymi danymi: Jan Kowalski (NIP 1234563218) wystawia Beispiel GmbH z Bawarii fakturę na 4000.00 EUR miesięcznie. Działalność ruszyła w czerwcu 2025. Ulga na start trwała do listopada 2025, składki preferencyjne do czerwca 2026, od lipca 2026 są pełne. W przykładach jest 25 listopada 2026, więc rok jeszcze trwa i program liczy też szacunek na koniec roku.

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

Blok `Where 2026 stands` pokazuje, za ile miesięcy roku jest faktura, wpłacony ryczałt i ZUS. Dopisek `+ Dec 2025` oznacza składki za miesiąc spoza roku (tu grudzień), wpłacone w tym roku. Są odliczane w tym roku. Wiersz `next` podaje najbliższy termin.

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

Wiersze rozliczenia, od góry:

| Wiersz | Co to jest |
|--------|------------|
| `revenue` | przychód z faktur, których data uzyskania przychodu wypada w roku, w PLN |
| `- social ZUS paid` | składki społeczne wpłacone w roku, bez FP/FS, za dowolny miesiąc |
| `- 50% of health ZUS paid` | połowa zapłaconych w roku składek zdrowotnych |
| `= taxable base` | przychód minus odliczenia, zaokrąglony do pełnych złotych |
| `ryczalt 12%` | podatek od podstawy, w pełnych złotych |
| `- ryczalt already paid` | zaliczki wpłacone za miesiące roku, razem z grudniową |
| `= to pay` albo `= refund` | to, co zostaje do zapłaty albo do zwrotu |

Odliczenia nie przekroczą przychodu. Nadwyżka przepada. Terminy w nagłówku (`filed 2027-02-15 to 2027-04-30`) to okres od 15 lutego do 30 kwietnia następnego roku, przesunięty z weekendu lub święta. Dopóki rok trwa, wiersz `year-end estimate` zakłada, że każdy kolejny miesiąc przyniesie tyle co ostatni zafakturowany i zostanie opłacony w terminie.

## Roczne rozliczenie składki zdrowotnej

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

| Wiersz | Co to jest |
|--------|------------|
| `revenue` | przychód roku |
| `- social ZUS paid` | wpłacone składki społeczne |
| `= yearly base` | przychód minus społeczne, w pełnych złotych. Decyduje o przedziale: do 60 000 PLN, do 300 000 PLN albo powyżej |
| `due for the year` | 9% z liczby ubezpieczonych miesięcy razy miesięczna podstawa przedziału |
| `- billed month by month` | suma składek miesięcznych, w seriach o tej samej kwocie |
| `= to pay` albo `= overpaid` | dopłata do 20 maja następnego roku (z dopiskiem, które miesiące rozliczono niżej) albo nadpłata, o której zwrot trzeba wnioskować |

Przedział wyznacza przychód całego roku, więc jeden miesiąc więcej lub mniej przesuwa wszystkie miesiące. W bieżącym roku `status` i `year` ostrzegają, gdy szacunek końca roku leży od progu bliżej niż jeden miesiąc przychodu, po dowolnej stronie. Termin 20 maja to termin ZUS DRA za kwiecień, z którą składa się rozliczenie.

## Pola formularzy: `ryczalt forms`

`forms` podaje to samo, ale pole po polu. Numery pól PIT-28(28) pochodzą z definicji formularza MF (crd.e-instytucja.pl, pozycja 14369), a bloku XII ZUS DRA z Poradnika ZUS DRA z maja 2026. Są przypisane tylko dla stawki 12%: przy innej `forms` ostrzega, że kwoty trzeba stosować z uwagą.

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

| Pole | Co wpisać |
|------|-----------|
| `P_4` | rok podatkowy |
| `P_30` | przychód z własnej działalności, 12% (część D, wiersz 1) |
| `P_57`, `P_63` | ten sam przychód, łącznie |
| `P_98` | wpłacone składki społeczne |
| `P_105` | połowa wpłaconych składek zdrowotnych (art. 11 ust. 1a ustawy o ryczałcie) |
| `P_137` | przychód po odliczeniach |
| `P_202` | podstawa opodatkowania, pełne złote |
| `P_217`, `P_232` | ryczałt 12%, należny |
| `P_236` | ryczałt zapłacony, czyli zaliczki za miesiące roku |
| `P_237` albo `P_238` | do zapłaty albo nadpłata |
| `P_239-50` | ryczałt za każdy miesiąc, tak jak zapłacony (część Q) |

Pola `P_100`, `P_104`, `P_107`, `P_114`, `P_125`, `P_177` i `P_224` do `P_230` wynikają z powyższych, a resztę program uznaje za 0.

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

Blok XII ZUS DRA za kwiecień:

| Pole | Co wpisać |
|------|-----------|
| `01` | rok rozliczeniowy |
| `21` | forma opodatkowania: ryczałt (`X`) |
| `22` | przychód pomniejszony o wpłacone składki społeczne |
| `23` | roczna podstawa: liczba miesięcy razy miesięczna podstawa przedziału |
| `24` | roczna składka: 9% pola 23 |
| `25` | suma składek miesięcznych, jak na miesięcznych DRA |
| `26` albo `27` | do zapłaty albo nadpłata |
| `28` albo `29` | to samo łącznie. Dopłata trafia też do bloków VI i IX.02, jednym przelewem na rachunek NRS. Zwrot nadpłaty wymaga wniosku (RZS-R) |

Pole `24` to 9% rocznej podstawy, nie suma dwunastu miesięcznych kwot, więc może się różnić o kilka groszy. Gdy rok nie jest zakończony, wszystkie kwoty mają przedrostek `~` i są szacunkiem: reszta roku jest liczona tak jak ostatni zafakturowany miesiąc. Uruchom `forms` ponownie, gdy rok będzie zafakturowany.

Kwoty w przykładzie to szacunek na dzień 25 listopada. Dla roku, który jest już zafakturowany, `year` i `forms` wypisują stan `the year is complete` i kwoty bez `~`.

## Ewidencja przychodów: `ryczalt ewid`

Ewidencja przychodów roku (rozporządzenie MF z 6 września 2025, Dz.U. 2025 poz. 1294). Każdy przychód to jeden wiersz, w kolejności dat uzyskania przychodu. Data wpisu to dzień wystawienia faktury, a gdy data uzyskania przychodu jest późniejsza, ta data.

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

W terminalu z kolorami żółta komórka oznacza wpis po terminie, czyli po 20. dniu następnego miesiąca (§ 6 rozporządzenia), albo nabywcę bez numeru VAT. Pod tabelą jest legenda. W przykładzie ostatnia faktura była wystawiona 25 listopada za październik, więc jej wpis jest po terminie. Bez kolorów (przy przekierowanym wyjściu albo z `NO_COLOR`) te komórki mają na końcu `*`, a legenda zaczyna się od `*`, jak w przykładzie.

Z `--csv` polecenie wypisuje 17 kolumn rejestru z nagłówkami z rozporządzenia: kolumny stawek `17%` do `3%`, `Ogółem` i `Uwagi`. Kwota trafia do kolumny stawki z konfiguracji. W `Uwagach` jest przeliczenie waluty (kwota, kurs, tabela NBP i jej data).

```
Lp.,Data wpisu,Data uzyskania przychodu,Numer KSeF,Numer dowodu,Identyfikator podatkowy kontrahenta,17%,15%,14%,12.5%,12%,10%,8.5%,5.5%,3%,Ogółem,Uwagi
1,2026-02-01,2026-01-31,,FV-2026-02-001,DE123456789,,,,,16852.40,,,,,16852.40,"EUR 4000.00 x 4.2131, NBP 020/A/NBP/2026 of 2026-01-30"
2,2026-03-01,2026-02-28,,FV-2026-03-001,DE123456789,,,,,16893.20,,,,,16893.20,"EUR 4000.00 x 4.2233, NBP 040/A/NBP/2026 of 2026-02-27"
...
```

Z `--jpk` wypisuje plik JPK_EWP(4). Wymaga w konfiguracji sekcji `[jpk]` z kluczami `first_name`, `surname`, `birth_date` (`RRRR-MM-DD`) i `tax_office` (czterocyfrowy kod urzędu skarbowego). Klucz `email` jest opcjonalny. Bez wymaganych kluczy polecenie kończy się kodem 2 i wymienia brakujące. `--correction` zmienia cel złożenia z pierwszego pliku (`CelZlozenia` 1) na korektę (2). Testy programu sprawdzają plik ze schematem MF, jeśli jest `xmllint`.

Wynik trafia na standardowe wyjście, więc zapisujesz go przekierowaniem. Ostrzeżenia idą na stderr, nigdy do pliku:

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

Plik JPK_EWP(4) idzie razem z PIT-28 (tak opisuje go `ryczalt -h`). Program go nie wysyła.

## Zawieszenie działalności

Klucze `tax.suspended_from` i `tax.suspended_until` (pierwszy i ostatni miesiąc zawieszenia, bez `suspended_until` aż do odwołania) zmieniają liczby tak:

- miesiące zawieszenia nie mają składek: ani społecznych, ani zdrowotnej, ani FP/FS,
- nie liczą się do ubezpieczonych miesięcy, więc rozliczenie zdrowotne mnoży podstawę przez mniejszą liczbę miesięcy,
- brakująca faktura za taki miesiąc nie jest uchybieniem, a `status` pokazuje `suspended`,
- rok, w którym zawieszenie trwa do końca, jest uznany za zakończony, gdy zafakturowano wszystkie miesiące przed zawieszeniem,
- faktura z przychodem w miesiącu zawieszenia jest liczona, ale `status` i `year` ostrzegają o niej.

Zawieszenie liczy się pełnymi miesiącami. Poniżej ten sam rok z `suspended_from=2026-11`:

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

Miesięcy z faktur jest teraz 10 z 10, a roczna składka to 9% z 10 podstaw, nie z 12. Listopad wchodzi do zestawienia składek jako `Nov 0.00`.

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

## Jak to się układa w czasie

Terminy, które program sam podaje:

| Co | Termin |
|----|--------|
| faktura za miesiąc | 15. dzień następnego miesiąca (art. 106i ust. 1 ustawy o VAT) |
| ZUS i ryczałt za miesiąc | 20. dzień następnego miesiąca |
| wpis do ewidencji przychodów | 20. dzień miesiąca po uzyskaniu przychodu (§ 6 rozporządzenia) |
| PIT-28 | od 15 lutego do 30 kwietnia następnego roku |
| rozliczenie składki zdrowotnej | 20 maja następnego roku, z ZUS DRA za kwiecień |

Każdy z nich przesuwa się z soboty, niedzieli lub święta na następny dzień roboczy. Zasady przesuwania opisuje [Jak liczy](rules.md). Program nie podaje terminu złożenia pliku JPK_EWP.
