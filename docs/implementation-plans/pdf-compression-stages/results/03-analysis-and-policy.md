# Результат Stage 03: Аналіз PDF та політика MVP

- Статус: **Completed**.
- Останнє оновлення: 2026-10-09.
- Чат/виконавець: поточний chat розробки.
- Версія плану: 2026-10-09; редакція GS-v1; engine baseline — Go 1.27.2 + open-source Ghostscript 10.08.0 + qpdf 12.4.2.
- Baseline: Stage 02 adapters завершено; Git repository недоступний (`git status`: not a git repository).

## Фактичний результат

Реалізовано пакет `internal/pdf` для комплексного аналізу вхідних документів, суворої політики MVP, інвентаризації та семантичних снепшотів:
1. `internal/pdf/policy.go`: реалізовано `CheckPolicy`, що відхиляє спеціальні типи документів до виклику будь-яких важких інструментів (encryption, signatures, AcroForm/XFA, PDF/A у bounded XMP, вкладення/портфоліо, JavaScript/Launch/RichMedia, tagged PDF, optional content layers, непідтримувані типи анотацій, анотації Link із appearance `/AP` або `/AS`, а також нетипові дії посилань поза URI та GoTo).
2. `internal/pdf/inventory.go`: реалізовано `ExtractInventory` та `AssessGSEligibility`. Визначає всі досяжні XObject-зображення, дедуплікує їх за канонічним ID, виявляє колірні простори та фільтри, обчислює кількість пікселів із безпечною перевіркою на переповнення. Формує машинні попередження `complex_color_preserved` (CMYK/Separation/DeviceN/ICCBased), `unsupported_image_filter` (фільтри поза списком підтримки) та `image_pixel_limit` (>40 MP або overflow), які вимикають весь GS-pass, залишаючи доступним qpdf lossless fallback.
3. `internal/pdf/snapshot.go`: реалізовано `BuildSnapshot` та `CompareSnapshots`. Збирає фізичну геометрію сторінок (MediaBox, CropBox, Rotate, UserUnit з толерантністю 0.01 pt), посилання (URI та GoTo з регістрацією цільових сторінок) та закладки (Outlines). Посилання та закладки резолвляться у 1-based номери сторінок замість прямих object IDs, що гарантує інваріантність порівняння до перенумерації об'єктів (object renumbering). Реалізовано захист від циклів та обмеження глибини закладок (depth cap 32).
4. `internal/pdf/analyzer.go`: реалізовано `AnalyzeDocument` — повний конвеєр аналізу з викликом `qpdf.Client`: швидка перевірка шифрування, вилучення структурного JSON (ліміт 64 MiB), читання bounded потоку XMP (ліміт 4 MiB), перевірка політики, інвентаризація, оцінка придатності до GS та створення снепшоту.
5. Розширено `internal/qpdf/json.go`: додано поля `Attachments`, `Encrypt` та прапорці форми в `JSONOutput`, а також підтримку прямого розбору trailer dictionary.

## Змінені файли та прийняті рішення

- `internal/pdf/policy.go` — перевірка політики спеціальних типів (`CheckPolicy`, `PolicyResult`, коди відмов).
- `internal/pdf/inventory.go` — інвентар зображень, дедуплікація, перевірка ліміту 40 MP, оцінка GS-придатності.
- `internal/pdf/snapshot.go` — вилучення геометрії, посилань, закладок та інваріантне до renumbering порівняння.
- `internal/pdf/analyzer.go` — координуючий аналізатор `AnalyzeDocument`, методи перевірки `IsAllowed`, `IsEligibleForGS`.
- `internal/pdf/policy_test.go` — юніт-тести відхилення всіх спеціальних типів та дозволу валідних Link annotations.
- `internal/pdf/inventory_test.go` — юніт-тести підрахунку унікальних зображень, лімітів пікселів, фільтрів та кольорів.
- `internal/pdf/snapshot_test.go` — юніт-тести толерантності 0.01 pt та повної еквівалентності після object renumbering.
- `internal/pdf/analyzer_test.go` — юніт-тести повного конвеєра аналізу, включаючи валідацію на реальному фікстурі.
- `internal/qpdf/json.go` — оновлено `JSONOutput` (`Attachments`, `Encrypt`, `AcroFormInfo`) та метод `Trailer()`.
- `docs/test-dependency-audit.md` — зафіксовано аудит залежностей Stage 03 перед запуском unit-тестів.
- `docs/implementation-plans/pdf-compression-stages/README.md` — оновлено рядок Stage 03 у tracker на `Completed`.

## Перевірки

| Перевірка | Команда або ручна процедура | Фактичний результат | Артефакт / доказ |
|---|---|---|---|
| Unit dependencies audit | `./scripts/check-test-dependencies.sh` + manual review | Pass: 0 DB/network/external imports | `docs/test-dependency-audit.md` |
| Unit tests (all packages) | `make unit` (`go test -v ./...`) | Pass: 100% тестів успішно (exit 0) | вивід `make unit` |
| Special types refusal | `go test -v ./internal/pdf -run TestCheckPolicy` | Pass: 10/10 сценаріїв відхилено | `internal/pdf/policy_test.go` |
| Inventory & GS eligibility | `go test -v ./internal/pdf -run TestAssessGS` | Pass: точні попередження та ліміти | `internal/pdf/inventory_test.go` |
| Object renumbering invariance | `TestSnapshotObjectRenumberingInvariance` | Pass: снепшоти ідентичні після renumbering | `internal/pdf/snapshot_test.go` |
| Real probe fixture analysis | `TestAnalyzeDocumentRealProbeFixture` | Pass: успішний аналіз `jpeg-150dpi.pdf` | `internal/pdf/analyzer_test.go` |
| Docker probe integration | `make probe RUN_ID=stage03-test SECURITY_OPT=` | Pass: 16 observations, status Pass | `artifacts/stage-01/stage03-test-probe/report.json` |

## Критерії завершення

1. **Special types відмовляються до GS:** Доведено. Усі спеціальні типи (encryption, signatures, AcroForm/XFA, PDF/A у bounded XMP, вкладення, JS, tagged, layers, non-link анотації, link appearance, нетипові дії) відхиляються функцією `CheckPolicy` з точними кодами відмови до спроби переписування.
2. **Complex color/pixel/unknown cases мають точні eligibility warnings:** Доведено. `AssessGSEligibility` генерує машинні коди `complex_color_preserved`, `unsupported_image_filter` та `image_pixel_limit` (при перевищенні 40 MP або overflow), вимикаючи GS-pass і дозволяючи qpdf fallback.
3. **Geometry/links/bookmarks snapshots порівнюються після object renumbering:** Доведено. Тест `TestSnapshotObjectRenumberingInvariance` підтверджує, що оскільки GoTo/Outlines резолвляться в порядкові номери сторінок, повна заміна indirect object IDs у результуючому документі визнається еквівалентною з толерантністю 0.01 pt.
4. **Counts не видаються за optimized-image count:** Доведено. Документовано, що `UniqueReachableImagesCount` відображає лише досяжні image XObjects без урахування inline images (BI/ID/EI), ресурси яких захищені лімітами процесу. Жодних тверджень про кількість "оптимізованих" зображень не робиться.
5. **Жодних CTM parser або Go-resizer:** Доведено. Відсутні власні лексери content stream або зміна розміру зображень у Go; downsampling делеговано Ghostscript.

## Відхилення, дефекти та обмеження

1. Inline images у content streams сторінок не відображаються окремими об'єктами у JSON qpdf. За контрактом вони не включаються до лічильника reachable image objects і контролюються загальними таймаутами та процесорними лімітами середовища.
2. Робота з XMP обмежена лімітом 4 MiB (`qpdf.DefaultXMPLimit`); перевищення ліміту призводить до безпечної відмови обробки (`CodePDFA`).

## Передача наступному чату

- **Наступний етап:** Stage 04 — Профілі стиснення Ghostscript (`04-ghostscript-profiles.md`).
- **Готові інтерфейси:**
  - `internal/pdf.AnalyzeDocument(ctx, client, filePath)`: повертає `*DocumentAnalysis`
  - `internal/pdf.CheckPolicy(doc, xmpData)`: повертає `*PolicyResult`
  - `internal/pdf.ExtractInventory(doc)`: повертає `*DocumentInventory`
  - `internal/pdf.AssessGSEligibility(inv)`: повертає `*GSEligibility`
  - `internal/pdf.BuildSnapshot(doc)`: повертає `*DocumentSnapshot`
  - `internal/pdf.CompareSnapshots(orig, candidate)`: повертає `(bool, []string)`
- **Вимоги для Stage 04:** Реалізувати `internal/ghostscript/{profiles,settings,diagnostics}.go` із фіксованими balanced/compact PostScript словниками та парсингом діагностики.
