# Stage 07: Якість PDF та навантаження

Редакція: **GS-v1**, 2026-10-09. Статус: **Completed**.

## Вхідні матеріали

[Основний контракт](../2026-10-09-pdf-compression-service.md), [tracker](README.md), [decisions](decisions.md), локальні інструкції. Stage 06 Completed; прочитати його results та звірити artifacts/code.

## Мета

Незалежно перевірити GS переписування PDF та ресурсні ліміти.

## Файли у scope

`tests/integration/compress_test.go`, `tests/e2e/api_test.go`, full `testdata/pdf/`, `docs/quality-report.md`, `docs/benchmark-report.md`, QA tooling.

Локальний план виконання: [07-execution-plan.md](07-execution-plan.md). Користувач доручив створити план і виконати його в цьому чаті.

## Реалізація

Poppler незалежно від GS порівнює text/page order/geometry/renders, embedded/unembedded fonts і OCR, links/bookmarks, diagrams/QR/barcode, masks/inline/Form/shared placements. Lossless pixels exact, lossy appearance acceptable, mono decoded pixels unchanged. GS може міняти streams/IDs/codecs: не вимагати їх byte identity. Failed feature validation має truthful fallback. Codec matrix для JPEG/Flate/Indexed/JPX/CCITT/JBIG2 має фактичні записи. Benchmark1/2workers до500pages/50MiB, dedup overhead, processing time/RAM/disk, busy/cancel/cleanup; caps не послаблювати мовчки. Defect fixes мінімальні з targeted rerun.

## Критерії завершення

Acceptance fixture matrix і quality/benchmark reports з доказами; no unresolved failed requirement. Not run не позначати Pass. Ресурсні/якісні failures потребують виправлення чи documented blocker.

## Перевірки і докази

Actual commands/results/artifact paths, crops/manual review facts, perf measurements, audit перед кожним unit rerun, fixes і runtime-supported/fallback codec matrix.

Ніколи не запускати міграції. Перед кожним unit run перевірити dependencies/imports/helpers/init/TestMain/config на DB interaction/data writes, записати audit. Не вважати fake-tool unit test доказом real decoder support.

## Результат і handoff

Заповнити [results/07-quality-and-load.md](results/07-quality-and-load.md) фактичними changes/commands/results/artifacts/audit/deviations/interfaces. Оновити тільки свій tracker row. Completed лише з доказом кожного локального критерію; иначе In progress/Blocked з конкретною причиною. Не реалізовувати наступний stage у цьому чаті.

## Запит для окремого чату

```text
Реалізуй лише Stage 07 за docs/implementation-plans/pdf-compression-stages/07-quality-and-load.md.
Прочитай основний план GS-v1, tracker, decisions і результат попереднього stage; звір їх із кодом.
Спочатку короткий план цього stage, потім реалізація в межах scope.
Стек: Go + open-source Ghostscript + qpdf; API внутрішній без авторизації.
Не запускай міграції. Перед кожним unit run перевір dependencies на DB interaction/data writes і запиши audit.
Запиши фактичні results у results-файл цього stage, онови tracker, залиш interfaces/artifacts і handoff.
Не реалізовуй наступні stages і не створюй нові чати автоматично.
```
