# Stage 02: Adapters Ghostscript/qpdf та процеси

Редакція: **GS-v1**, 2026-10-09. Статус: **Not started**.

## Вхідні матеріали

[Основний контракт](../2026-10-09-pdf-compression-service.md), [tracker](README.md), [decisions](decisions.md), локальні інструкції. Stage 01 Completed; прочитати його results та звірити artifacts/code.

## Мета

Реалізувати bounded clients обох CLI і Linux process runner.

## Файли у scope

`internal/qpdf/{client,json,objects,streams}.go`; `internal/ghostscript/client.go`; `internal/worker/runner_linux.go`; qpdf json/streams/client tests, ghostscript client tests, runner tests; оновити probe.

## Реалізація

qpdf IsEncrypted/Check/Analyze/ReadStream/WriteLossless і GS Rewrite/ExtractText приймають trusted options/job paths. Без update-from-json або Go image decoding. Exit codes encryption відокремити, stdout diagnostic/JSON/text і stderr bounded, cancellation/deadline та process-group reap. Rewrite поки працює з baseline settings probe; профільний production builder належить Stage 04. Interfaces описати для analyzer/service; shared runner не створює нову process group кожному qpdf/gs child всередині pdfworker.

## Критерії завершення

Обидва adapters запускаються без shell; malformed/errors/caps коректно mapped; txtwrite 32 MiB cap; дочірнє дерево завершується при cancel; probe використовує adapters.

## Перевірки і докази

Unit dependency audit і tests, real gs/qpdf adapter smoke, exact args/errors/interfaces, child cleanup facts. Не додавати API, custom codecs або placement lexer.

Ніколи не запускати міграції. Перед кожним unit run перевірити dependencies/imports/helpers/init/TestMain/config на DB interaction/data writes, записати audit. Не вважати fake-tool unit test доказом real decoder support.

## Результат і handoff

Заповнити [results/02-tool-adapters.md](results/02-tool-adapters.md) фактичними changes/commands/results/artifacts/audit/deviations/interfaces. Оновити тільки свій tracker row. Completed лише з доказом кожного локального критерію; иначе In progress/Blocked з конкретною причиною. Не реалізовувати наступний stage у цьому чаті.

## Запит для окремого чату

```text
Реалізуй лише Stage 02 за docs/implementation-plans/pdf-compression-stages/02-tool-adapters.md.
Прочитай основний план GS-v1, tracker, decisions і результат попереднього stage; звір їх із кодом.
Спочатку короткий план цього stage, потім реалізація в межах scope.
Стек: Go + open-source Ghostscript + qpdf; API внутрішній без авторизації.
Не запускай міграції. Перед кожним unit run перевір dependencies на DB interaction/data writes і запиши audit.
Запиши фактичні results у results-файл цього stage, онови tracker, залиш interfaces/artifacts і handoff.
Не реалізовуй наступні stages і не створюй нові чати автоматично.
```
