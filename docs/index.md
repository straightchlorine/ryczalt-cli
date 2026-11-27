---
title: "Ryczałt, ZUS i terminy z wiersza poleceń"
description: "ryczalt czyta archiwum faktur i liczy co miesiąc przychód, ryczałt, składki ZUS i terminy. Zapisuje wpłaty, wysyła fakturę do KSeF, podaje kwoty do PIT-28."
---

# ryczalt-cli

Ryczałt, ZUS i terminy płatności dla jednoosobowej działalności, z wiersza
poleceń.

`ryczalt` czyta archiwum faktur (jeden folder `RRRR-MM` na miesiąc) i pokazuje
miesiąc po miesiącu przychód, ryczałt, składki ZUS oraz terminy płatności.

Zapisuje wpłaty, przygotowuje kolejną fakturę i wysyła ją do KSeF przez
[ksef-cli](https://codeberg.org/piotrkrzysztof/ksef-cli). Na koniec roku podaje
kwoty do PIT-28 i do rocznego rozliczenia składki zdrowotnej.

## Funkcje

- Przelicza przychód w walucie obcej po kursie NBP z ostatniego dnia
  roboczego przed dniem uzyskania przychodu (art. 11a ustawy o PIT)
- Liczy ryczałt narastająco od początku roku, składki społeczne według etapu
  (ulga na start, preferencyjne, pełne) i składkę zdrowotną według progu
- Przesuwa terminy wypadające w weekend lub święto, z polskimi i niemieckimi
  kalendarzami świąt
- Zapisuje wpłaty (`ryczalt paid`) i pokazuje, co jest zapłacone, a co zaległe
- Przygotowuje fakturę na kolejny miesiąc jako plik
  [Invoify](https://github.com/al1abb/invoify) skopiowany z poprzedniej faktury
- Wysyła fakturę do KSeF przez [ksef-cli](https://codeberg.org/piotrkrzysztof/ksef-cli)
  i kopiuje archiwum do R2 lub innego S3
- Podaje kwoty do PIT-28 i szacuje roczne rozliczenie składki zdrowotnej, także
  pole po polu do PIT-28 i bloku XII DRA (`ryczalt forms`)
- Uwzględnia zawieszenie działalności
- Ostrzega, gdy roczny przychód zbliża się do progu składki zdrowotnej
- Prowadzi ewidencję przychodów: tabela, CSV lub plik JPK_EWP(4) sprawdzony
  oficjalnym schematem MF
- Sprawdza archiwum (`ryczalt check`): brakujące numery KSeF, faktury
  wystawione po terminie, kontrahent z UE zidentyfikowany inaczej niż numerem
  VAT

## Instalacja

Potrzebny jest Go 1.26 lub nowszy.

```bash
go install codeberg.org/piotrkrzysztof/ryczalt-cli/cmd/ryczalt@latest
```

Polecenie instaluje program `ryczalt` w `~/go/bin`. Gotowe pliki wykonywalne są
na [stronie wydań](https://codeberg.org/piotrkrzysztof/ryczalt-cli/releases)
na Codeberg.

Wysyłka faktur wymaga zainstalowanego [ksef-cli](https://codeberg.org/piotrkrzysztof/ksef-cli).
Pozostałe polecenia go nie potrzebują.

## Jak zacząć?

```bash
ryczalt init ~/invoices    # tworzy ~/.config/ryczalt.conf (NIP z ksef.conf)
$EDITOR ~/.config/ryczalt.conf
ryczalt status             # bieżący rok, miesiąc po miesiącu
ryczalt year               # PIT-28 i roczne rozliczenie składki zdrowotnej
```

`init` nigdy nie nadpisuje istniejącego pliku. Opis kluczy jest na stronie
[Konfiguracja](configuration.md), a układ folderów na stronie
[Archiwum faktur](archive.md).

## Co miesiąc

```bash
ryczalt new                         # faktura za poprzedni miesiąc (invoice.json)
#  Invoify: import, sprawdzenie, eksport PDF
ryczalt add -m ~/Downloads/FV.pdf   # PDF do folderu miesiąca
ryczalt send                        # wysyłka do KSeF przez ksef-cli, kopia do R2
ryczalt paid 2026-11                # po zapłaceniu ZUS i ryczałtu
```

Wszystkie polecenia i ich opcje opisuje strona [Polecenia](commands.md).

## Dla kogo?

Obecna wersja obsługuje jeden przypadek: ryczałt od przychodów
ewidencjonowanych, zwolnienie z VAT (art. 113) i zaliczki miesięczne. Inne
ustawienia w pliku konfiguracji program odrzuca.

Kwoty składek na dany rok są wpisane w programie wraz ze źródłem. Każdy rok
wymaga aktualizacji w styczniu, gdy GUS i ZUS publikują nowe wartości.

Program nie rozstrzyga kwestii prawnych. Czego jeszcze nie robi, opisuje strona
[O projekcie](about.md).
