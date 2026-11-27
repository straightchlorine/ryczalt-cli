---
title: "Archiwum faktur: układ folderów i pliki"
description: "Jak ułożyć archiwum faktur dla ryczalt: folder RRRR-MM na miesiąc, pliki invoice.json, FA(3) i UPO, które program czyta, co zapisuje i czego nie nadpisuje."
---

# Archiwum faktur

Archiwum to zwykły folder na dysku, wskazany kluczem `archive` w
[konfiguracji](configuration.md). Program czyta z niego faktury i zapisuje w nim
kilka plików. Żadnej bazy danych nie ma.

## Układ folderów

Każdy miesiąc ma własny folder o nazwie `RRRR-MM`, np. `2026-11`. Foldery
o innych nazwach program pomija. Folder miesiąca może być dowiązaniem
symbolicznym do innego miejsca.

`ryczalt new` zapisuje fakturę w folderze miesiąca wystawienia, a `ryczalt add`
domyślnie wkłada pliki do folderu miesiąca wystawienia najnowszej faktury.
Czytając archiwum, program nie bierze daty z nazwy folderu, tylko z samej
faktury.

```
~/invoices/
|-- 2026-10/
|   |-- invoice.json
|   |-- ksef-FV-2026-10.xml
|   |-- ksef-FV-2026-10.upo.xml
|   `-- FV-2026-10.pdf
|-- 2026-11/
|   `-- invoice.json
`-- 2026/
    `-- payments.laptop.csv
```

Folder `RRRR` bez miesiąca, np. `2026`, trzyma wpłaty z danego roku.

## Co program czyta

W folderze miesiąca czyta tylko pliki `*.xml`, `invoice.json` i `.ksef/meta.json`.
Reszta (PDF, umowy, pokwitowania) jest dla niego obojętna. Pliki większe niż
10 MB pomija.

| Plik                    | Skąd i co program z niego bierze |
|-------------------------|----------------------------------|
| `invoice.json`          | Eksport z Invoify, tylko bezpośrednio w folderze miesiąca. Numer, data wystawienia, waluta i kwota. Numer VAT kontrahenta z pola dodatkowego odbiorcy o nazwie `VAT`, `NrVatUE`, `VAT ID` lub `NIP`. Koniec usługi to ostatni dzień miesiąca przed datą wystawienia. |
| `*.xml` z elementem `Faktura` | Faktura FA(3), w folderze miesiąca lub w jego podfolderze. Numer (`P_2`), data wystawienia (`P_1`), koniec usługi (`P_6_Do`, potem `P_6`, potem `P_1`), waluta, kwota (`P_15`), NIP sprzedawcy i identyfikator nabywcy. |
| `*.xml` z elementem `Potwierdzenie` | UPO z KSeF. Numer faktury, numer KSeF i tryb wysyłki (`Online` albo `Offline`). |
| `.ksef/meta.json`       | Starszy układ: numer faktury i numer KSeF. Gdy jest też UPO, wygrywa UPO. |

### Co z tego wynika

- Numer KSeF z UPO lub z `.ksef/meta.json` oznacza, że faktura poszła do KSeF.
  `ryczalt send` wysyła tylko szkice `invoice.json` bez takiego numeru.
  Fakturę bez numeru, wystawioną od 1 kwietnia 2026, zgłasza `ryczalt check`.
- Gdy ta sama faktura jest jako `invoice.json` i jako FA(3), program liczy
  z XML. Jeśli kwota, waluta lub okres się różnią, `ryczalt status` ostrzega.
  Sama inna data wystawienia szkicu to tylko uwaga w `ryczalt check`.
- Ta sama faktura w dwóch plikach tego samego rodzaju (dwa `invoice.json` albo dwa XML) z identycznymi danymi liczy się raz.
  Gdy dane się różnią, program przerywa z błędem.
- Faktura wystawiona przez inny NIP niż `tax.nip` nie jest przychodem i jest
  pomijana, np. faktura zakupu pobrana z KSeF.
- Faktura FA(3) innego rodzaju niż zwykła (`RodzajFaktury` różne od `VAT`)
  albo z kwotą VAT większą od zera przerywa pracę z błędem. Korekt i faktur
  zaliczkowych program jeszcze nie obsługuje.
- Plik XML, którego program nie umie przeczytać, jest pominięty z ostrzeżeniem.
- Klucz sesji KSeF zapisany jawnym tekstem w `.ksef/meta.json` (`aes_key_hex`)
  zgłasza `ryczalt check`.

## Co program zapisuje

| Polecenie       | Co powstaje                                                          |
|-----------------|----------------------------------------------------------------------|
| `ryczalt new`   | `RRRR-MM/invoice.json` w folderze miesiąca wystawienia, kopia poprzedniej faktury z nowym numerem, datami, kursem NBP i kwotą w PLN. Tworzy folder, jeśli go brak. |
| `ryczalt add`   | Kopie podanych plików w folderze miesiąca (`--month` albo domyślny). Z `--move` program usuwa potem oryginały. |
| `ryczalt send`  | Wywołuje `ksef convert --send` na `invoice.json`. Obok niego ksef-cli zapisuje `ksef-NUMER.xml`, a po wysłaniu `ksef-NUMER.upo.xml`. W `NUMER` znaki spoza `A-Z`, `a-z`, `0-9`, `.`, `_` i `-` zamieniają się na `-`. |
| `ryczalt paid`, `ryczalt void` | Wiersze w `RRRR/payments.NAZWA-KOMPUTERA.csv`, gdzie `RRRR` to rok daty zapłaty (dla `void` - rok dnia anulowania). |
| `ryczalt sync`  | Nic w archiwum. Wypycha je do bucketu z `[r2]`. Pliki Syncthing (`.stfolder`, `.stignore`, `.stversions`, kopie konfliktów) pomija. |

`ryczalt init` zapisuje tylko plik konfiguracji, nic w archiwum. Kursy NBP
program trzyma poza archiwum, w `$XDG_CACHE_HOME/ryczalt` albo `~/.cache/ryczalt`.

### Plik wpłat

Plik `payments.NAZWA-KOMPUTERA.csv` ma nagłówek i jeden wiersz na wpłatę:

```
id,paid_on,period,kind,amount,note
20261105-a1b2c3,2026-11-05,2026-10,ryczalt,1000.00,
```

- `period` to miesiąc, którego dotyczy wpłata.
- `kind` to `social`, `health`, `fpfs` albo `ryczalt`. Wiersz `void` anuluje
  wpłatę o id podanym w ostatniej kolumnie.
- Każdy komputer zapisuje do własnego pliku, żeby narzędzie do synchronizacji
  (np. Syncthing) nie widziało dwóch maszyn piszących w ten sam plik.
- Plik z błędnym nagłówkiem lub wierszem program pomija w całości i ostrzega.
  Plik `payments.*` o innej nazwie, np. kopię konfliktu synchronizacji,
  pomija i prosi o ręczne scalenie.

## Czego program nie nadpisuje

- `ryczalt new`, `ryczalt add` i `ryczalt init` odmawiają, gdy plik docelowy
  już istnieje.
- `ryczalt add` sprawdza wszystkie pliki przed kopiowaniem, więc błąd nie
  zostawia pracy w połowie. Kopiuje z zachowaniem czasu modyfikacji.
- `ryczalt send` odmawia, gdy plik `ksef-NUMER.upo.xml` już istnieje.
- Wpłat nie edytuje ani nie usuwa. Pomyłkę anuluje `ryczalt void`, który
  dopisuje wiersz.
- `ryczalt sync` niczego nie pobiera z R2. Kasuje tam pliki tylko z opcją
  `--delete`.

Polecenia opisuje strona [Polecenia](commands.md).
