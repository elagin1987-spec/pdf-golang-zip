# Результат Stage 05: Конвеєр та перевірка кандидатів

- Статус: **Completed**.
- Останнє оновлення: 2026-10-09.
- Чат/виконавець: поточний chat розробки.
- Версія плану: 2026-10-09; редакція GS-v1; engine baseline — Go 1.27.2 + open-source Ghostscript 10.08.0 + qpdf 12.4.2.
- Baseline: Stage 04 Ghostscript profiles завершено; Git repository недоступний (`git status`: not a git repository).

## Фактичний результат

Реалізовано виробничий конвеєр стиснення документів, валідацію кандидатів та механізм truthful selection:
1. `internal/compression/profiles.go`: реалізовано типи та валідацію профілів `ProfileBalanced` ("balanced", 300 DPI за замовчуванням), `ProfileCompact` ("compact", 150 DPI) та `ProfileLossless` ("lossless"). Реалізовано функцію `ParseProfile(raw_name)` (case-insensitive, default balanced) та метод `(p Profile) ToGSProfile()`, який повертає помилку `ErrLosslessCannotMapToGS` для lossless-профілю (гарантія, що lossless ніколи не викликає `pdfwrite`).
2. `internal/compression/result.go`: визначено структури результату `Result`, статуси `StatusCompressed` і `StatusUnchanged`, рушії `EngineOriginal`, `EngineQPDF` і `EngineGhostscript`, машинні коди попереджень (`complex_color_preserved`, `unsupported_image_filter`, `image_pixel_limit`, `ghostscript_failed`, `ghostscript_warning`, `ghostscript_text_changed`, `ghostscript_features_changed`, `text_validation_unavailable`) та функцію дедуплікації попереджень зі збереженням порядку `DeduplicateWarnings`.
3. `internal/compression/validation.go`: реалізовано функцію `ValidateCandidate`:
   - Виконує обов'язкову структурну перевірку `qpdf --check` (exit 0).
   - Будує та порівнює семантичні снепшоти оригіналу та кандидата через `pdf.CompareSnapshots`: перевіряє рівність кількості сторінок, еквівалентність фізичних рамок (MediaBox, CropBox, Rotate, UserUnit з точністю 0.01 pt), посилань (URI та GoTo) і закладок. При невідповідності повертає `WarningGhostscriptFeaturesChanged`.
   - Для кандидатів Ghostscript виконує екстракцію нормалізованого тексту через `txtwrite` (CRLF/CR->LF, trim trailing spaces, trim terminal blank lines) та звірку з текстом оригіналу. При зміні тексту повертає `WarningGhostscriptTextChanged`, при помилці екстракції чи перевищенні 32 MiB — `WarningTextValidationUnavailable`.
   - Для lossless-кандидатів qpdf виклик Ghostscript текстової екстракції не здійснюється.
   - Підраховує унікальні reachable image XObjects кандидата для вихідних заголовків.
4. `internal/compression/service.go`: реалізовано `Service` та метод `Compress(ctx, req)`:
   - Перевіряє вхідні параметри та наявність робочої директорії.
   - Проводить preflight: перевірку оригіналу через `qpdf.Check` та повний аналіз `pdf.AnalyzeDocument`. При відхиленні політикою повертає типізовану помилку `PolicyViolationError`.
   - Запускає генерацію qpdf lossless кандидата (`lossless.pdf`) та валідує його.
   - За умови доступності документа (`IsEligibleForGS()`) та профілю balanced/compact виконує Ghostscript rewrite (`gs-candidate.pdf`).
   - Перевіряє діагностику `Diagnostics` (exit code, SAFER, font substitutions, warnings). При виявленні зауважень Ghostscript кандидат відкидається з фіксацією `WarningGhostscriptFailed` або `WarningGhostscriptWarning`.
   - При чистому переписуванні виконує текстову й семантичну валідацію, а також формує додаткового кандидата структурної оптимізації `gs-structural.pdf` через qpdf без повторного стиснення зображень.
   - Порівнює розміри всіх валідних кандидатів з оригіналом. Якщо жоден кандидат не дав зменшення розміру (або розміри однакові), повертається `StatusUnchanged`, `EngineOriginal`, точний вихідний шлях оригіналу (`OutputPath = req.InputPath`) та 0.0% економії. Якщо валідний кандидат менший за оригінал, вибирається найменший із кандидатів.
   - Окремий шлях для обов'язкових помилок: `context.Canceled`, `context.DeadlineExceeded`, відмова preflight або відхилення за політикою повертають явні помилки і не приховуються fallback'ом.
5. `internal/compression/profiles_test.go`, `validation_test.go`, `service_test.go`, `test_helpers_test.go`: юніт-тести покривають усі сценарії парсингу профілів, валідації геометрії/посилань/тексту, відхилення GS за попередженнями діагностики, truthful selection найменшого кандидата, поведінку unchanged, complex color skip та mandatory errors.
6. `tests/integration/compress_test.go`: реальні інтеграційні тести з викликом реальних бінарників qpdf і Ghostscript у Docker-середовищі на фікстурах `flate-rgb-600dpi.pdf`, `jpeg-600dpi.pdf`, `mono-flate-600dpi.pdf` та `jpeg-150dpi.pdf`.

## Змінені файли та прийняті рішення

- `internal/compression/profiles.go` — визначення профілів, валідація та перетворення в параметри Ghostscript.
- `internal/compression/profiles_test.go` — юніт-тести парсингу профілів та перевірки заборони мапінгу lossless на Ghostscript.
- `internal/compression/result.go` — структури результатів, константи статусів, рушіїв та попереджень.
- `internal/compression/validation.go` — повноцінна валідація кандидатів (qpdf check, snapshot comparison, txtwrite text comparison).
- `internal/compression/validation_test.go` — юніт-тести валідації геометрії, розбіжностей сторінок, лінків та тексту.
- `internal/compression/service.go` — координуючий сервіс стиснення з truthful candidate selection.
- `internal/compression/service_test.go` — юніт-тести конвеєра для balanced, compact, lossless, fallback, unchanged та обов'язкових помилок.
- `internal/compression/test_helpers_test.go` — тестові хелпери та `fakeRunner` для ізольованого тестування.
- `tests/integration/compress_test.go` — інтеграційні тести повного конвеєра на реальних фікстурах з перевіркою реального стиснення.
- `docs/test-dependency-audit.md` — зафіксовано аудит залежностей Stage 05 перед виконанням тестів.
- `docs/implementation-plans/pdf-compression-stages/README.md` — оновлено рядок Stage 05 у tracker на `Completed`.

## Перевірки

| Перевірка | Команда або ручна процедура | Фактичний результат | Артефакт / доказ |
|---|---|---|---|
| Unit dependencies audit | `./scripts/check-test-dependencies.sh` + manual review | Pass: 0 DB/network/external imports | `docs/test-dependency-audit.md` |
| Host unit tests (all packages) | `make unit` (`go test -v ./...`) | Pass: 100% тестів успішно (exit 0) | вивід `make unit` |
| Compression package unit tests | `go test -v ./internal/compression/...` | Pass: 13 з 13 тестів успішно (exit 0) | вивід тестувальника Go |
| Real isolated integration tests | `docker run ... test -v ./tests/integration/...` | Pass: 4 з 4 тестів успішно (exit 0) | вивід контейнера QA |
| Docker probe validation | `make probe RUN_ID=stage05-probe SECURITY_OPT=` | Pass: 24 observations, exit 0 | `artifacts/stage-01/stage05-probe-probe/report.json` |
| Docker QA integration | `make integration RUN_ID=stage05-qa SECURITY_OPT=` | Pass: 24 observations, exit 0 | `artifacts/stage-01/stage05-qa-qa/report.json` |

## Критерії завершення

1. **Усі профілі й smallest/unchanged працюють:** Доведено. Профілі balanced, compact та lossless успішно генерують відповідних кандидатів; при відсутності виграшу в розмірі повертається точний оригінал із статусом `StatusUnchanged` та engine `EngineOriginal`.
2. **GS failure/warning/features/text changes відкидаються:** Доведено. У юніт-тестах `TestServiceCompressGSWarningFallbackToQPDF` та `TestServiceCompressGSTextMismatchFallbackToOriginal` кандидат Ghostscript негайно відкидається з фіксацією відповідного коду попередження.
3. **Valid qpdf/original fallback:** Доведено. За відсутності виграшу або при відхиленні GS кандидата конвеєр безпомилково повертає валідний qpdf lossless кандидат або точний оригінал.
4. **Mandatory errors мають окремий шлях:** Доведено. `TestServiceCompressMandatoryErrors` підтверджує, що помилки preflight, зашифровані документи, порушення політики та скасування контексту повертають явні помилки (`ErrPreflightFailed`, `PolicyViolationError`, `context.Canceled`), а не маскуються під fallback.
5. **Жодного повторного lossy pass:** Доведено. Ghostscript запускається виключно один раз безпосередньо з вхідного `input.pdf`.

## Відхилення, дефекти та обмеження

1. Для деяких файлів (наприклад, `jpeg-600dpi.pdf`) переписування Ghostscript у збалансованому режимі з високою якістю QFactor 0.4 збільшує розмір файлу порівняно з оригіналом (227 KB vs 111 KB). Конвеєр реалізує truthful selection: у збалансованому режимі перемагає qpdf lossless (111.7 KB), тоді як у компактному режимі (150 DPI) перемагає Ghostscript (65.5 KB).
2. Заборона запускати міграції та повна ізоляція від БД дотримана на 100%.

## Передача наступному чату

- **Наступний етап:** Stage 06 — HTTP API та робочий контейнер (`06-http-and-worker.md`).
- **Готові інтерфейси для Stage 06:**
  - `compression.Profile`: `ProfileBalanced`, `ProfileCompact`, `ProfileLossless`, `ParseProfile(string) (Profile, error)`
  - `compression.Result`: поля `Status`, `Engine`, `OriginalBytes`, `OutputBytes`, `SavedPercent`, `Warnings`, `InputImageObjects`, `OutputImageObjects`, `OutputPath`, допоміжна функція `DeduplicateWarnings`
  - `compression.Service`: `NewService(qpdfClient, gsClient) *Service`, `Service.Compress(ctx, CompressRequest) (*Result, error)`
  - `compression.CompressRequest`: `InputPath`, `WorkDir`, `Profile`
  - `compression.PolicyViolationError`: структуровані коди та причини відмов за політикою MVP
- **Вимоги для Stage 06:** Побудувати HTTP-сервер, обробник `POST /v1/compress` з multipart streaming, endpoints `/healthz` та `/readyz`, lifecycle робочих директорій у `/work/jobs`, мапінг помилок на коди HTTP (400, 413, 415, 422, 503, 504, 500) та заголовки відповіді.
