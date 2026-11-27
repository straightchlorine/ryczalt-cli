---
title: "O projekcie ryczalt-cli i jego ograniczenia"
description: "Czego ryczalt-cli nie robi, jak ułożony jest kod i jak uruchomić testy, także sprawdzenie JPK_EWP przez xmllint. Licencja MIT, autor i repozytorium na Codeberg."
---

# O projekcie

## Czego narzędzie nie robi

- Obsługuje jeden przypadek: ryczałt od przychodów ewidencjonowanych, zwolnienie
  z VAT (art. 113) i zaliczki miesięczne. Inne ustawienia w `tax.form`,
  `tax.payment` i `tax.vat_status` program odrzuca przy wczytaniu konfiguracji.
- Zna jedną stawkę ryczałtu naraz, wpisaną w `tax.rate`.
- Niczego nie składa. PIT-28, plik JPK_EWP i DRA przygotowuje się z jego
  kwot, a składa samodzielnie przez oficjalne portale. Do KSeF wysyła tylko
  fakturę, przez ksef-cli.
- Nie wykonuje przelewów. `ryczalt paid` tylko zapisuje wpłatę.
- Nie tworzy pliku PDF z fakturą. Robi to Invoify, a `ryczalt new` przygotowuje
  dla niego `invoice.json`.
- `ryczalt new` działa tylko dla faktur w walucie obcej. Kopiuje poprzednią
  fakturę, więc pierwszą trzeba przygotować samodzielnie.
- Nie obsługuje faktur korygujących ani zaliczkowych. Taka faktura w archiwum
  przerywa pracę z błędem.
- Nie jest doradztwem podatkowym. Program nie rozstrzyga kwestii prawnych,
  np. wyboru `revenue_basis` albo stawki. Kwoty składek na dany rok trzeba
  aktualizować co roku, a każda wartość ma w programie podane źródło
  (`ryczalt rules`).

## Rozwój

Cały kod leży w `cmd/ryczalt/` jako jeden pakiet `main` i korzysta wyłącznie
z biblioteki standardowej Go.

```bash
go test ./...
```

Test pliku JPK_EWP sprawdza go oficjalnym schematem MF z `testdata/` za pomocą
`xmllint`. Bez `xmllint` ten fragment testu jest pomijany.

## Licencja

Licencja MIT. Szczegóły w pliku [LICENSE](https://codeberg.org/piotrkrzysztof/ryczalt-cli/src/branch/main/LICENSE).

## Autor

Piotr Krzysztof Lis, [piotrkrzysztof.dev](https://piotrkrzysztof.dev). Kod źródłowy jest na [Codeberg](https://codeberg.org/piotrkrzysztof/ryczalt-cli), a jego kopia na [GitHubie](https://github.com/straightchlorine/ryczalt-cli).
