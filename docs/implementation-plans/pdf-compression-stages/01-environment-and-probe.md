# Stage 01: Середовище та перевірка Ghostscript

Редакція: **GS-v1**, 2026-10-09. Статус: **Completed**; фактичні докази та Stage 07 follow-ups — у results-файлі.

## Вхідні матеріали

[Основний контракт](../2026-10-09-pdf-compression-service.md), [tracker](README.md), [decisions](decisions.md), локальні інструкції. Попереднього implementation stage немає. Ghostscript обраний користувачем; стек не вибирати наново.

## Мета

Побудувати Docker із open-source gs/qpdf та перевірити fixed-profile pdfwrite, txtwrite і qpdf lossless на синтетичних PDF.

## Файли у scope

`go.mod`, `go.sum`, `Dockerfile` probe/QA targets, `.dockerignore`, `THIRD_PARTY_NOTICES.md`, `cmd/engine-probe/main.go`, `testdata/pdf/`, `docs/engine-evaluation.md`, `Makefile`, `scripts/check-test-dependencies.sh`, `docs/test-dependency-audit.md`.

## Реалізація

Зафіксувати GS-v1 versions/checksums, AGPL і notices/fonts/resources. Probe без server binaries. Known-placement JPEG/Flate 600 dpi → GS 300 dpi; 150 dpi без апскейлу; txtwrite original/output; qpdf check/json/lossless. Перевірити mono, JPX і JBIG2 positive fixtures, якщо codecs доступні у selected build. SAFER має доступ тільки до потрібних fixed job paths. Перевірити link/bookmark rendering/extraction; якщо GS їх втрачає, зафіксувати потребу semantic fallback у pipeline, а не приховано видалити. Не створювати Go resizer або JSON update generator.

## Критерії завершення

Зафіксовані gs/qpdf запускаються без activation key; 300-dpi probe valid, text/geometry контрольовані, positive format matrix має фактичні результати. Якщо version/artifact/API недоступні, Blocked із доказом без silent replacement.

## Перевірки і докази

Докази versions/checksums, exact profile argv/settings, original/output bytes і dimensions, independent Poppler renders/text, codec Pass/Fail і engine-evaluation. Тести лише після dependency audit.

Ніколи не запускати міграції. Перед кожним unit run перевірити dependencies/imports/helpers/init/TestMain/config на DB interaction/data writes, записати audit. Не вважати fake-tool unit test доказом real decoder support.

## Результат і handoff

Заповнити [results/01-environment-and-probe.md](results/01-environment-and-probe.md) фактичними changes/commands/results/artifacts/audit/deviations/interfaces. Оновити тільки свій tracker row. Completed лише з доказом кожного локального критерію; иначе In progress/Blocked з конкретною причиною. Не реалізовувати наступний stage у цьому чаті.

## Запит для окремого чату

```text
Реалізуй лише Stage 01 за docs/implementation-plans/pdf-compression-stages/01-environment-and-probe.md.
Прочитай основний план GS-v1, tracker, decisions і результат попереднього stage; звір їх із кодом.
Спочатку короткий план цього stage, потім реалізація в межах scope.
Стек: Go + open-source Ghostscript + qpdf; API внутрішній без авторизації.
Не запускай міграції. Перед кожним unit run перевір dependencies на DB interaction/data writes і запиши audit.
Запиши фактичні results у results-файл цього stage, онови tracker, залиш interfaces/artifacts і handoff.
Не реалізовуй наступні stages і не створюй нові чати автоматично.
```
