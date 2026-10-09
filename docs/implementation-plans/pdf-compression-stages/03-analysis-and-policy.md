# Stage 03: Аналіз PDF та політика MVP

Редакція: **GS-v1**, 2026-10-09. Статус: **Not started**.

## Вхідні матеріали

[Основний контракт](../2026-10-09-pdf-compression-service.md), [tracker](README.md), [decisions](decisions.md), локальні інструкції. Stage 02 Completed; прочитати його results та звірити artifacts/code.

## Мета

Зібрати document inventory, special-type refusal, GS eligibility і semantic snapshots без CTM parser.

## Файли у scope

`internal/pdf/{analyzer,policy,inventory,snapshot}.go`; policy/inventory/snapshot tests; additional fixtures.

## Реалізація

AnalyzeDocument/CheckPolicy/AssessGSEligibility/BuildSnapshot читають qpdf JSON і bounded XMP. Відмови encryption/signatures/forms/PDF-A/attachments/JS/tagged/layers. Inventory filters/BPC/ColorSpace/masks/pixels; unknown filter, complex colors чи >40 MP вимикають весь GS-pass, але допускають qpdf fallback. Порахувати unique reachable images. Snapshot physical pages/links/GoTo/URI/outlines незалежно від refs; depths/cycles/caps. Inline images не вичерпуються JSON inventory: не обіцяти точний count усіх inline occurrences, їх ресурси контролюють process limits. DPI/placement виконує GS, не власний lexer.

## Критерії завершення

Special types відмовляються до GS; complex color/pixel/unknown cases мають точні eligibility warnings; geometry/links/bookmarks snapshots порівнюються після object renumbering; counts не видаються за optimized-image count.

## Перевірки і докази

Unit audit, negative fixtures, real qpdf inventory/snapshots і semantic equivalence cases. Вказати межі inventory для inline images. Не додавати CTM/resizer.

Ніколи не запускати міграції. Перед кожним unit run перевірити dependencies/imports/helpers/init/TestMain/config на DB interaction/data writes, записати audit. Не вважати fake-tool unit test доказом real decoder support.

## Результат і handoff

Заповнити [results/03-analysis-and-policy.md](results/03-analysis-and-policy.md) фактичними changes/commands/results/artifacts/audit/deviations/interfaces. Оновити тільки свій tracker row. Completed лише з доказом кожного локального критерію; иначе In progress/Blocked з конкретною причиною. Не реалізовувати наступний stage у цьому чаті.

## Запит для окремого чату

```text
Реалізуй лише Stage 03 за docs/implementation-plans/pdf-compression-stages/03-analysis-and-policy.md.
Прочитай основний план GS-v1, tracker, decisions і результат попереднього stage; звір їх із кодом.
Спочатку короткий план цього stage, потім реалізація в межах scope.
Стек: Go + open-source Ghostscript + qpdf; API внутрішній без авторизації.
Не запускай міграції. Перед кожним unit run перевір dependencies на DB interaction/data writes і запиши audit.
Запиши фактичні results у results-файл цього stage, онови tracker, залиш interfaces/artifacts і handoff.
Не реалізовуй наступні stages і не створюй нові чати автоматично.
```
