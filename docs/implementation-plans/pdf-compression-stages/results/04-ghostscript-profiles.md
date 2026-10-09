# Результат Stage 04: Профілі стиснення Ghostscript

- Статус: **Completed**.
- Останнє оновлення: 2026-10-09.
- Чат/виконавець: поточний chat розробки.
- Версія плану: 2026-10-09; редакція GS-v1; engine baseline — Go 1.27.2 + open-source Ghostscript 10.08.0 + qpdf 12.4.2.
- Baseline: Stage 03 analysis and policy завершено; Git repository недоступний (`git status`: not a git repository).

## Фактичний результат

Реалізовано виробничі профілі стиснення Ghostscript, безпечну генерацію PostScript-словників дистилятора та структурований розбір діагностики виконання:
1. `internal/ghostscript/settings.go`: реалізовано `BuildDistillerSettings(s ProfileSettings)` для генерації надійного рядка параметрів дистилятора (`setdistillerparams`, `NeverEmbed []`, `/PreserveAnnotTypes [/Link] def`, `ColorACSImageDict`, `GrayACSImageDict` з точними QFactor і підвибірками). Специфікація виключає будь-яке проникнення користувацького PostScript-коду; параметри є суто серверними.
2. `internal/ghostscript/profiles.go`: реалізовано типізовані профілі `ProfileBalanced`, `ProfileCompact`, `ProfileLossless`, структури `ProfileSettings` та функцію побудови точного масиву CLI-аргументів `BuildArgs(profile, inputPath, outputPath)`:
   - `balanced`: 300 DPI (Color/Gray), Bicubic, поріг 1.1, QFactor 0.4, sampling [1 1 1 1], mono downsample false.
   - `compact`: 150 DPI (Color/Gray), Bicubic, поріг 1.1, QFactor 0.76, chroma subsampling [2 1 1 2] для кольору та [1 1 1 1] для сірого, mono downsample false.
   - `lossless`: виклик `BuildArgs` повертає `ErrLosslessNoRewriteArgs`, гарантуючи відсутність виклику `pdfwrite` для lossless-профілю.
   - Спільні прапорці: `-dBATCH`, `-dNOPAUSE`, `-dSAFER`, `-sDEVICE=pdfwrite`, `-dCompatibilityLevel=1.7`, `-dAutoRotatePages=/None`, `-sColorConversionStrategy=LeaveColorUnchanged`, `-dConvertCMYKImagesToRGB=false`, `-dPassThroughJPEGImages=true`, `-dPassThroughJPXImages=true`, `-dEmbedAllFonts=true`, `-dSubsetFonts=true`, `-dCompressFonts=true`, `-dCompressStreams=true`, `-dDetectDuplicateImages=true`, `-dFastWebView=false`, `-dPreserveAnnots=true`, а також дозволи `--permit-file-read` і `--permit-file-write`.
3. `internal/ghostscript/diagnostics.go`: реалізовано розширену структуру `Diagnostics` та функцію `ParseDiagnostics(exitCode, stdout, stderr)`. Виявляє порушення пісочниці SAFER (`invalidfileaccess`), підміну шрифтів (`Substituting font`), попередження Ghostscript (`Warning:`), фатальні помилки (`Error:`, `Unrecoverable error`) та сигналізує прапорцем `SuggestedFallback` для конвеєра валідації Stage 05.
4. `internal/ghostscript/client.go`: інтегровано підтримку профілю через `RewriteOptions.Profile`. При виклику `Rewrite` автоматично будуються аргументи профілю та повертається розібрана `Diagnostics`. Збережено повну зворотну сумісність для `BuildBaselineBalancedArgs` та раніше створених тестів.
5. `cmd/engine-probe/main.go`: розширено для тестування кандидата `compact` поруч із `lossless` та `balanced`. Усі 24 спостереження (8 фікстур × 3 кандидати) успішно верифіковано в ізольованому Docker-середовищі.

## Змінені файли та прийняті рішення

- `internal/ghostscript/settings.go` — генерація безпечного рядка PostScript дистилятора (`BuildDistillerSettings`).
- `internal/ghostscript/settings_test.go` — юніт-тести точного формування рядка PostScript для balanced і compact профілів.
- `internal/ghostscript/profiles.go` — визначення профілів, параметрів, валідації та побудови CLI-аргументів (`BuildArgs`).
- `internal/ghostscript/profiles_test.go` — юніт-тести точних значень аргументів, перевірки DPI, заборони для lossless.
- `internal/ghostscript/diagnostics.go` — структурований парсинг діагностики Ghostscript (`ParseDiagnostics`).
- `internal/ghostscript/diagnostics_test.go` — юніт-тести виявлення SAFER, підміни шрифтів, попереджень та помилок.
- `internal/ghostscript/client.go` — оновлено `RewriteOptions`, інтегровано виклик `BuildArgs` та парсинг діагностики.
- `internal/ghostscript/client_test.go` — додано тести виклику `Rewrite` для compact та відхилення lossless.
- `cmd/engine-probe/main.go` — додано кандидата `compact` до циклу перевірки інтеграційного зонду.
- `docs/test-dependency-audit.md` — зафіксовано аудит залежностей Stage 04 перед запуском unit-тестів.
- `docs/engine-evaluation.md` — зафіксовано результати калібрування профілів та заміри байтів/роздільності для 24 кандидатів.
- `docs/implementation-plans/pdf-compression-stages/README.md` — оновлено статус Stage 04 на `Completed`.

## Перевірки

| Перевірка | Команда або ручна процедура | Фактичний результат | Артефакт / доказ |
|---|---|---|---|
| Unit dependencies audit | `./scripts/check-test-dependencies.sh` + manual review | Pass: 0 DB/network/external imports | `docs/test-dependency-audit.md` |
| Unit tests (all packages) | `make unit` (`go test -v ./...`) | Pass: 100% тестів успішно (exit 0) | вивід `make unit` |
| Settings PostScript exactness | `go test -v ./internal/ghostscript -run TestBuildDistillerSettings` | Pass: точні параметри та інваріанти | `internal/ghostscript/settings_test.go` |
| Profiles argv exactness | `go test -v ./internal/ghostscript -run TestBuildArgs` | Pass: повний збіг прапорців та заборона lossless | `internal/ghostscript/profiles_test.go` |
| Diagnostics mapping | `go test -v ./internal/ghostscript -run TestParseDiagnostics` | Pass: виявлення SAFER, font sub, warnings | `internal/ghostscript/diagnostics_test.go` |
| Docker probe (compact + balanced) | `make probe RUN_ID=stage04-compact-test SECURITY_OPT=` | Pass: 24 observations, exit 0 | `artifacts/stage-01/stage04-compact-test-probe/report.json` |
| Docker QA integration | `make integration RUN_ID=stage04-qa-test SECURITY_OPT=` | Pass: 24 observations, Poppler text/render Pass | `artifacts/stage-01/stage04-qa-test-qa/report.json` |

## Критерії завершення

1. **Exact argv/settings проходять unit tests:** Доведено. Аргументи CLI та рядки PostScript повністю відповідають контракту GS-v1; відсутні неявні пресети `/screen` чи спрощені прапорці якості.
2. **600-dpi fixtures стають target 300/150:** Доведено. У тестах та реальному зонді `jpeg-600dpi.pdf`, `flate-rgb-600dpi.pdf`, `flate-gray-600dpi.pdf`, `jpx-600dpi.pdf` стають 600×600 (300 DPI) для `balanced` та 300×300 (150 DPI) для `compact`.
3. **150dpi не upsamples:** Доведено. Для `jpeg-150dpi.pdf` та `flate-rgb-150dpi.pdf` зображення залишаються 300×300 (150 DPI) в обох профілях; поріг 1.1 запобігає апскейлу.
4. **Mono без loss:** Доведено. Прапорець `-dDownsampleMonoImages=false` у всіх профілях зберігає розміри монохромних зображень 1200×1200 без втрат або даунсемплінгу.
5. **Color/codec matrix truthful:** Доведено. Реальні фіксури для DCTDecode, FlateDecode, JBIG2Decode та JPXDecode успішно проходять переписування у зафіксованій збірці Ghostscript 10.08.0.
6. **Профіль lossless не має Rewrite args:** Доведено. `BuildArgs(ProfileLossless, ...)` повертає типізовану помилку `ErrLosslessNoRewriteArgs`.

## Відхилення, дефекти та обмеження

1. Відповідно до архітектури GS-v1, профіль `lossless` обслуговується виключно qpdf (перестиснення потоків flate та об'єктні потоки) і ніколи не викликає Ghostscript.
2. У Poppler render для монохромних фікстур зберігається відома особливість відображення країв повносторінкового кропу (передано як gate для Stage 07); при цьому попіксельна тотожність декодованих монохромних пікселів підтверджена на 100%.

## Передача наступному чату

- **Наступний етап:** Stage 05 — Конвеєр та перевірка кандидатів (`05-compression-pipeline.md`).
- **Готові інтерфейси:**
  - `internal/ghostscript.ProfileBalanced`, `ProfileCompact`, `ProfileLossless`
  - `internal/ghostscript.BuildArgs(profile, inputPath, outputPath)` -> `([]string, error)`
  - `internal/ghostscript.GetProfileSettings(profile)` -> `(ProfileSettings, error)`
  - `internal/ghostscript.BuildDistillerSettings(settings)` -> `string`
  - `internal/ghostscript.ParseDiagnostics(exitCode, stdout, stderr)` -> `Diagnostics`
  - `internal/ghostscript.Client.Rewrite(ctx, in, out, RewriteOptions{Profile: ...})` -> `(Diagnostics, error)`
- **Вимоги для Stage 05:** Побудувати координуючий конвеєр стиснення, що поєднує аналіз `internal/pdf`, виконання qpdf lossless, виклик Ghostscript за обраним профілем та сувору валідацію/вибір найменшого валідного кандидата з надійним fallback.
