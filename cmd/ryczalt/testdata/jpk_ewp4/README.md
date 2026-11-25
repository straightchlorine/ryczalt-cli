# JPK_EWP(4) schemas

Official Ministry of Finance schemas, used by `ewid_test.go` to validate
`ryczalt ewid --jpk`. Downloaded 2026-11-25.

| File | Source | SHA-256 of the original |
|---|---|---|
| `Schemat_JPK_EWP(4)_v1-0.xsd` | [gov.pl](https://www.gov.pl/attachment/67b55c59-e05c-42f0-be4c-28afcca460b6) ([page](https://www.gov.pl/web/kas/struktury-jpk-w-podatkach-dochodowych)) | `0061478a2af63894bf2ea6293eaf6ebd3da18a26ab43076899a20d3c3f32a79b` |
| `StrukturyDanych_v10-0E.xsd` | [crd.gov.pl](http://crd.gov.pl/xml/schematy/dziedzinowe/mf/2022/01/05/eD/DefinicjeTypy/StrukturyDanych_v10-0E.xsd) | `1137ce6e3c11c2b9ef3f05e4e72d6dcd6b4fa94908ea558f2ba15de0259bb2aa` |
| `ElementarneTypyDanych_v10-0E.xsd` | [crd.gov.pl](http://crd.gov.pl/xml/schematy/dziedzinowe/mf/2022/01/05/eD/DefinicjeTypy/ElementarneTypyDanych_v10-0E.xsd) | `8a531cb181d3e298d11b28766655ae91fee2d7851440095932ffc82137ed2be1` |
| `KodyKrajow_v10-0E.xsd` | [crd.gov.pl](http://crd.gov.pl/xml/schematy/dziedzinowe/mf/2022/01/05/eD/DefinicjeTypy/KodyKrajow_v10-0E.xsd) | `1d41a1b3184188f2d20a51d3afde26204dda182ec5dacf018204dcc9870dc644` |
| `KodyUrzedowSkarbowych_v8-0E.xsd` | [crd.gov.pl](http://crd.gov.pl/xml/schematy/dziedzinowe/mf/2022/01/05/eD/KodyUrzedowSkarbowych/KodyUrzedowSkarbowych_v8-0E.xsd) | `af27f583ea1ceb44ed37e0ec0d9bfdf2bfda74aadd5841ccbf0fc241175c2e91` |

The only change: imports point at the local files, so validation works offline.

```sh
sed -i 's#schemaLocation="http://crd.gov.pl/xml/schematy/dziedzinowe/mf/2022/01/05/eD/[A-Za-z]*/#schemaLocation="#' *.xsd
```
