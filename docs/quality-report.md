# Stage 07 — quality report

Повторна перевірка поточної збірки після policy/XMP/qpdf fixes:
[96/96 Pass і незалежні докази](policy-validation-fixes-report.md).
Наведений нижче Stage 07 звіт збережено як історичний.

Дата: 2026-10-09. **Pass** для локальної fixture matrix GS-v1.
Ghostscript 10.08.0, qpdf 12.4.2, Poppler 24.02.0, zbar 0.23.93;
independent pypdf 6.10.0. Production settings/caps не змінено.

## Матриця прийняття та докази

32 owned synthetic fixtures × lossless/balanced/compact = **96 Pass**:
69 успішних PDF та 27 policy refusals до GS rewrite. Повна матриця/checks/розміри/engine —
[quality.json](../artifacts/stage-07/quality-accepted/quality.json), SHA-256/provenance —
[quality-manifest](../testdata/pdf/quality-manifest.json), [fixtures README](../testdata/pdf/README.md).
Реальний API повторив усі 96 cases; integration перевірив geometry/links/bookmarks.
[Лог](../artifacts/stage-07/real-tests-accepted.log), exit 0, no skipped tests.

| Критерій | Результат і незалежний доказ |
|---|---|
| qpdf strict validity; returned <= original; unchanged exact bytes | Pass для всіх 69 успішних cases; QA та API assertions |
| Lossless render pixels | Pass, **23/23 exact** Poppler page sets; GS не обирається для lossless |
| Текст, OCR і порядок сторінок | Pass: Poppler `pdftotext -layout`, Unicode/internal whitespace/form-feeds збережені; 3-page Cyrillic/OCR fixture |
| Фізична геометрія, URI/GoTo, bookmarks | Pass, **69/69** independent pypdf snapshots без production helper; Poppler pdfinfo geometry також збігається |
| Embedded/unembedded fonts | Pass: DejaVu Cyrillic embedded subset retained; Helvetica/Courier unembedded input стають embedded Type1C у GS; pdffonts evidence |
| Vectors, 6pt text, masks/SMask, inline/nested Form/shared/mirrored placements | Pass: numerical renders + manual reviewed full pages/contact sheets; no clipping/black boxes/lost placements |
| QR/Code128 | Pass: zbar читає `STAGE07-QUALITY-2026` / `STAGE07-128` у всіх трьох профілях, Code128 на всіх 3 сторінках |
| Mono | Pass: **12/12 returned** decoded-image equality; **8/8 raw GS** equality. CCITT/JBIG2/Flate незалежно декодуються в однакову матрицю |
| Known DPI/no upscale | Pass: raw GS color/gray 1200px → 600/300px на 144pt placement; 300px input не збільшується; mono 1200px лишається |
| Failed-feature validation | Pass: UserUnit/Rotate/CropBox fixture повертає qpdf з `ghostscript_features_changed`, exact renders, а не змінений GS |
| Complex/unsupported policy fallback | Pass: CMYK/ICC `complex_color_preserved`; ASCIIHex image `unsupported_image_filter`; engine qpdf/original |
| Special documents | Pass: AcroForm, XFA, signature, attachments, JS, tagged, layers, non-Link annotation, PDF/A declaration — 422 до переписування |

Independent features: [independent-features.json](../artifacts/stage-07/quality-accepted/independent-features.json)
і [лог](../artifacts/stage-07/independent-features.log).
Mono cross-codec: [hashes](../artifacts/stage-07/quality-accepted/mono-codec-crosscheck.json).
Fonts: `quality-accepted/text-ocr-vectors/{source,balanced/selected,compact/selected}/fonts.txt`.
Команди/exit/stderr: [quality-commands.json](../artifacts/stage-07/quality-accepted/quality-commands.json).

## Фактична codec matrix

Всі positive codecs реально декодовано зафіксованими tools; fake unit tests не є цим доказом.
`qpdf/original` у selected column означає найменший valid result, а не відсутність GS decoder.
GS raw candidates та їхні structural/text validations збережені навіть коли вони більші.

| Codec / fixture | GS decoder | balanced selected | compact selected | Доказ |
|---|---|---|---|---|
| JPEG/DCT 600dpi | Pass | qpdf | ghostscript | dimensions 600/300px у raw GS; render/text |
| JPEG/DCT 150dpi | Pass | qpdf | qpdf | raw 300px unchanged dimensions, no upscale |
| RGB/Gray Flate 600dpi | Pass | ghostscript | ghostscript | known DPI + renders |
| RGB Flate 150dpi | Pass | ghostscript | ghostscript | no upscale |
| Flate PNG predictor | Pass | qpdf | qpdf | predictor-gray returned exact; clean raw GS |
| Indexed RGB/Flate | Pass | ghostscript | ghostscript | numeric + reviewed pattern appearance |
| JPX/JPEG2000 | Pass | ghostscript | ghostscript | real lossless input decoder, 600/300px output; не обіцяє JPX encoder |
| CCITT Group4 | Pass | original | original | raw GS та returned decoded pixels exact |
| JBIG2 immediate MMR region | Pass | qpdf | qpdf | real decoded pixels == mono Flate, raw GS exact |
| LZW Gray | Pass | qpdf | qpdf | clean raw GS; returned exact |
| RunLength Gray | Pass | ghostscript | ghostscript | clean raw GS, appearance/text |
| ASCIIHex Gray | Policy fallback | qpdf | qpdf | explicit unsupported_image_filter, GS не запускається |
| CMYK / ICCBased | Policy fallback | original / qpdf | original / qpdf | explicit complex_color_preserved, exact renders |

Bounded fixture support: JBIG2 symbol dictionaries/global segments, every ICC profile,
all arbitrary documents і real scanner/device acceptance **Not run**, не позначені Pass.
Це межі corpus, не нові заяви про universal support. JBIG2 scope успадкований від Stage 01.

## Метод та ручний огляд

Poppler renders — 144 dpi, всі сторінки. Lossless/qpdf/original вимагають exact pixels.
GS lossy: MAE <= 6/255 як screening, плюс незалежні text/geometry/symbol/mono checks
і ручний огляд relevant images. MAE не є повним доказом fidelity.

Compact Indexed high-contrast pattern має MAE **6.451/255**. На full renders зміни
зосереджені на downsampling/chroma edges, pattern/layout/кольори лишаються прийнятними
для 150dpi. Для цього fixture явно заданий screening **8/255** у manifest; інші
thresholds, lossless equality та production profiles не змінені. Це записане калібрування
QA, не тихе ослаблення контракту. Maximum text/vector fixture MAE ~0.429/255.

Ручні факти/paths: [manual-review.md](../artifacts/stage-07/manual-review.md).
Reviewed PNGs побітово збігаються з accepted run:
[manual-review-equivalence.json](../artifacts/stage-07/manual-review-equivalence.json).
Кирилиця/6pt digits, lines/circles, QR/barcode, transparency, shared rotations та crop
переглянуті. OCR доведено extraction, а не видимістю hidden text.

## Виправлення та невдалі спроби

- Invalid direct AcroForm field у negative fixture → valid indirect object.
- CCITT TIFF/PDF polarity → explicit BlackIs1=true; independent cross-codec hash тепер exact.
- Max-load PDF padding after EOF → padding before xref, corrected startxref; valid PDF рівно 50MiB.
- Indexed screening calibrated після manual review, без production change.

`quality-run1` і `quality-final` — failed attempts, **не acceptance**; останній мав 96 Pass
rows, але failed cross-codec hash. Остаточний `quality-accepted.log`: **quality Pass**, exit 0.
Поточних unresolved failed local requirements немає. Production Go files не змінено.

## Відтворення

Fixtures: bundled Python with Pillow/reportlab/pypdf + DejaVu font:

```sh
python3 testdata/pdf/generate_quality.py
make quality-check SECURITY_OPT= DOCKER=/snap/docker/current/bin/docker
python3 scripts/check_quality_features.py artifacts/stage-07/quality-<RUN_ID>
```

Порожній SECURITY_OPT — тільки documented snap-host compatibility; default Makefile
no-new-privileges лишається. На сумісному host використовувати default.
Реальні acceptance commands/paths і audit — у [Stage 07 results](implementation-plans/pdf-compression-stages/results/07-quality-and-load.md).
