# Test data

- [`jpk_ewp4/`](jpk_ewp4/README.md): the official JPK_EWP(4) schemas.
- `holidays.txt`: public holidays for Poland and the 16 German states, 2025-2030,
  checked by `TestHolidaysGolden`. Generated from date.nager.at and feiertage-api.de,
  verified against the
  [Polish holidays act](https://api.sejm.gov.pl/eli/acts/DU/2025/296/text.pdf) and the
  German Foreign Office's
  [2026 list per state](https://www.bmeia.gv.at/fileadmin/user_upload/Vertretungen/Berlin/02_Dokumente/Deutsche_gesetzliche_Feiertage_2026.pdf).
  Holidays only some municipalities keep (e.g. 15 August in Catholic Bavaria) are
  left out; a calendar adds them with `+MM-DD`.
