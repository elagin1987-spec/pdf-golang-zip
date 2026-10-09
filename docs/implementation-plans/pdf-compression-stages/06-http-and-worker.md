# Stage 06: HTTP API та робочий контейнер

Редакція: **GS-v1**, 2026-10-09. Статус: **Completed**.

## Вхідні матеріали

[Основний контракт](../2026-10-09-pdf-compression-service.md), [tracker](README.md), [decisions](decisions.md), локальні інструкції. Stage 05 Completed; прочитати його results та звірити artifacts/code.

## Мета

Надати внутрішній API без авторизації з pdfworker/temp lifecycle.

## Файли у scope

`internal/storage/temp.go`, `internal/worker/protocol.go`, `cmd/pdfworker/main.go`, config/httpapi/server/observability files, OpenAPI/compose/env/README; runtime Docker target; http/storage/worker tests.

## Реалізація

Контракт GS-v1: streaming multipart, profiles, X-Optimization-Engine і input/output image-object counts; без optimized/skipped counter, який GS не гарантує. Два slots до upload, child group обох CLI, binary після validation, timeout/cancel/temp cleanup. Ready gs+qpdf, при ENABLE_LOSSY=false лише qpdf. Docker включає fonts/resources/licensing gs, readonlyroot і writable job paths, internal port bind. Без login/tokens/auth middleware/DB. Логи bytes/time/engine/warning codes без PDF content.

## Критерії завершення

Container API без credentials повертає valid compressed/unchanged/fallback PDF; status codes/headers/ready/busy/cancel працюють; gs/qpdf children і temp прибираються.

## Перевірки і докази

Unit audit, local e2e requests/headers, cleanup/process evidence, logs, exact runtime image та startup commands. Широкий benchmark лише Stage07.

Ніколи не запускати міграції. Перед кожним unit run перевірити dependencies/imports/helpers/init/TestMain/config на DB interaction/data writes, записати audit. Не вважати fake-tool unit test доказом real decoder support.

## Результат і handoff

Заповнити [results/06-http-and-worker.md](results/06-http-and-worker.md) фактичними changes/commands/results/artifacts/audit/deviations/interfaces. Оновити тільки свій tracker row. Completed лише з доказом кожного локального критерію; иначе In progress/Blocked з конкретною причиною. Не реалізовувати наступний stage у цьому чаті.

## Запит для окремого чату

```text
Реалізуй лише Stage 06 за docs/implementation-plans/pdf-compression-stages/06-http-and-worker.md.
Прочитай основний план GS-v1, tracker, decisions і результат попереднього stage; звір їх із кодом.
Спочатку короткий план цього stage, потім реалізація в межах scope.
Стек: Go + open-source Ghostscript + qpdf; API внутрішній без авторизації.
Не запускай міграції. Перед кожним unit run перевір dependencies на DB interaction/data writes і запиши audit.
Запиши фактичні results у results-файл цього stage, онови tracker, залиш interfaces/artifacts і handoff.
Не реалізовуй наступні stages і не створюй нові чати автоматично.
```

## План виконання цього чату

1. Config і private temp lifecycle, isolated pdfworker IPC та inherited tool group.
2. Streaming multipart HTTP, admission до upload, deadlines/cancel/shutdown, metadata і safe logs.
3. Pinned runtime target, Compose з internal bind/read-only/nonroot/limits, OpenAPI/README.
4. Dependency audit перед кожним unit run; unit/race, real-container API requests,
   unchanged/fallback, busy/timeout/disconnect та process/temp cleanup evidence.
5. Results та лише row Stage 06; Stage 07 не починати.
