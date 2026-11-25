<h1 align="center">ryczalt-cli</h1>

<div align="center">

[English](README.en.md) &middot; [Codeberg](https://codeberg.org/piotrkrzysztof/ryczalt-cli) &middot; [GitHub](https://github.com/straightchlorine/ryczalt-cli)

Ryczałt, ZUS i terminy płatności dla jednoosobowej działalności, z wiersza poleceń.

</div>

<br>

`ryczalt` czyta archiwum faktur (jeden folder `RRRR-MM` na miesiąc) i pokazuje
miesiąc po miesiącu przychód, ryczałt, składki ZUS oraz terminy płatności.
Zapisuje wpłaty, przygotowuje kolejną fakturę, wysyła ją do KSeF przez
[ksef-cli](https://codeberg.org/piotrkrzysztof/ksef-cli), a na koniec roku
podaje kwoty do PIT-28 i rocznego rozliczenia składki zdrowotnej.

## Funkcje

- Przelicza przychód w walucie obcej po kursie NBP z ostatniego dnia
  roboczego przed dniem uzyskania przychodu (art. 11a ustawy o PIT)
- Liczy ryczałt narastająco od początku roku, składki społeczne według etapu
  (ulga na start, preferencyjne, pełne) i składkę zdrowotną według progu
- Przesuwa terminy wypadające w weekend lub święto, z polskimi i niemieckimi
  kalendarzami świąt
- Zapisuje wpłaty (`ryczalt paid`) i pokazuje, co jest zapłacone, a co zaległe
- Przygotowuje fakturę na kolejny miesiąc jako plik
  [Invoify](https://github.com/al1abb/invoify) z poprzedniego
- Wysyła fakturę do KSeF przez [ksef-cli](https://codeberg.org/piotrkrzysztof/ksef-cli) i kopiuje
  archiwum do R2 lub innego S3
- Podaje kwoty do PIT-28 i szacuje roczne rozliczenie składki zdrowotnej, także
  pole po polu do PIT-28 i bloku XII DRA (`ryczalt forms`)
- Uwzględnia zawieszenie działalności
- Ostrzega, gdy roczny przychód zbliża się do progu składki zdrowotnej
- Prowadzi ewidencję przychodów: tabela, CSV lub plik JPK_EWP(4) sprawdzony
  oficjalnym schematem MF
- Sprawdza archiwum (`ryczalt check`): brakujące numery KSeF, faktury
  wystawione po terminie, kontrahent bez numeru VAT

## Jak zacząć?

Potrzebny jest Go 1.26 lub nowszy.

```bash
go install codeberg.org/piotrkrzysztof/ryczalt-cli/cmd/ryczalt@latest

ryczalt init ~/faktury     # tworzy ~/.config/ryczalt.conf (NIP z ksef.conf)
$EDITOR ~/.config/ryczalt.conf
ryczalt status             # bieżący rok, miesiąc po miesiącu
ryczalt year               # PIT-28 i roczne rozliczenie składki zdrowotnej
```

Co miesiąc:

```bash
ryczalt new                         # faktura za poprzedni miesiąc (invoice.json)
#  Invoify: import, sprawdzenie, eksport PDF
ryczalt add -m ~/Downloads/FV.pdf   # PDF do folderu miesiąca
ryczalt send                        # wysyłka do KSeF przez ksef-cli, kopia do R2
ryczalt paid 2026-11                # po zapłaceniu ZUS i ryczałtu
```

## Dla kogo?

Obecna wersja obsługuje jeden przypadek: ryczałt od przychodów
ewidencjonowanych, zwolnienie z VAT (art. 113) i zaliczki miesięczne.

Kwoty składek na dany rok są wpisane w programie wraz ze źródłem. Każdy rok
wymaga aktualizacji w styczniu, gdy GUS i ZUS publikują nowe wartości.

---

<div align="center">

Bogu samemu - cześć i chwała na wieki wieków.

</div>
