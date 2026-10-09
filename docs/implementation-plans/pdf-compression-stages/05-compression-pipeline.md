# Stage 05: Конвеєр та перевірка кандидатів

Редакція: **GS-v1**, 2026-10-09. Статус: **Not started**.

## Вхідні матеріали

[Основний контракт](../2026-10-09-pdf-compression-service.md), [tracker](README.md), [decisions](decisions.md), локальні інструкції. Stage 04 Completed; прочитати його results та звірити artifacts/code.

## Мета

Зібрати qpdf lossless і GS lossy candidates з truthful fallback/selection.

## Файли у scope

`internal/compression/{service,profiles,result,validation}.go`; service/validation tests; початковий `tests/integration/compress_test.go`.

## Реалізація

Compress preflight/policy/inventory → lossless original-based → eligible gs rewrite від original → optional qpdf structural finalization без image recompression → validation → valid smallest/original. Lossless ніколи не pdfwrite. Validate qpdf check, physical geometry, normalized extracted text GS txtwrite і links/outlines semantics. Не порівнювати content-stream hashes після GS. Source txtwrite недоступний/capped або candidate text/features змінені — reject GS і fallback. Timeout/cancel/required preflight failure не приховувати успіхом. Result engine/counts/warnings правдиві; exact original bytes при unchanged.

## Критерії завершення

Усі профілі й smallest/unchanged працюють; GS failure/warning/features/text changes відкидаються; valid qpdf/original fallback; mandatory errors мають окремий шлях; жодного повторного lossy pass.

## Перевірки і докази

Unit audit і real isolated gs/qpdf integration, text/feature difference regression, input/output sizes/hash, warnings, deadline/cancel. Release visual fidelity доводиться Stage07, не лише text equality.

Ніколи не запускати міграції. Перед кожним unit run перевірити dependencies/imports/helpers/init/TestMain/config на DB interaction/data writes, записати audit. Не вважати fake-tool unit test доказом real decoder support.

## Результат і handoff

Заповнити [results/05-compression-pipeline.md](results/05-compression-pipeline.md) фактичними changes/commands/results/artifacts/audit/deviations/interfaces. Оновити тільки свій tracker row. Completed лише з доказом кожного локального критерію; иначе In progress/Blocked з конкретною причиною. Не реалізовувати наступний stage у цьому чаті.

## Запит для окремого чату

```text
Реалізуй лише Stage 05 за docs/implementation-plans/pdf-compression-stages/05-compression-pipeline.md.
Прочитай основний план GS-v1, tracker, decisions і результат попереднього stage; звір їх із кодом.
Спочатку короткий план цього stage, потім реалізація в межах scope.
Стек: Go + open-source Ghostscript + qpdf; API внутрішній без авторизації.
Не запускай міграції. Перед кожним unit run перевір dependencies на DB interaction/data writes і запиши audit.
Запиши фактичні results у results-файл цього stage, онови tracker, залиш interfaces/artifacts і handoff.
Не реалізовуй наступні stages і не створюй нові чати автоматично.
```
