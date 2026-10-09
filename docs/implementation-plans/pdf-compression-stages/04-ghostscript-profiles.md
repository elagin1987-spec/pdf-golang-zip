# Stage 04: Профілі стиснення Ghostscript

Редакція: **GS-v1**, 2026-10-09. Статус: **Not started**.

## Вхідні матеріали

[Основний контракт](../2026-10-09-pdf-compression-service.md), [tracker](README.md), [decisions](decisions.md), локальні інструкції. Stage 03 Completed; прочитати його results та звірити artifacts/code.

## Мета

Зафіксувати balanced/compact args і довірені PostScript settings з вимірюваною якістю.

## Файли у scope

`internal/ghostscript/{profiles,settings,diagnostics}.go`; profiles/settings/diagnostics tests; розширити probe і fixtures; docs/engine-evaluation.md.

## Реалізація

ProfileSettings/BuildArgs/ParseDiagnostics за основним планом GS-v1: 300/150 dpi, bicubic, threshold1.1, mono downsample false, auto filter, QFactor .4/.76, sampling, fonts/NeverEmbed, color unchanged, passthrough, dedup і PreserveAnnotTypes. Ніяких /screen presets чи JPEG quality85/75. Settings лише серверні, без user snippets. Lost links/warnings/text differences пізніше відкидає Stage05 validation. Fixtures із shared usage/Form/rotation/UserUnit/inline/masks доводять behavior GS. Профіль lossless не має Rewrite args. Calibration changes записувати як явне відхилення з before/after доказом.

## Критерії завершення

Exact argv/settings проходять unit tests; 600-dpi fixtures стають target300/150, 150dpi не upsamples; mono без loss; рядки/QR/crops прийнятні; color/codec matrix truthful. Unknown mask/codec failure не називати supported.

## Перевірки і докази

Unit audit/results, known-placement PDF analysis і crops, calibrated settings, diagnostics mapping і вимоги fallback. Не додавати own codecs/image replacement.

Ніколи не запускати міграції. Перед кожним unit run перевірити dependencies/imports/helpers/init/TestMain/config на DB interaction/data writes, записати audit. Не вважати fake-tool unit test доказом real decoder support.

## Результат і handoff

Заповнити [results/04-ghostscript-profiles.md](results/04-ghostscript-profiles.md) фактичними changes/commands/results/artifacts/audit/deviations/interfaces. Оновити тільки свій tracker row. Completed лише з доказом кожного локального критерію; иначе In progress/Blocked з конкретною причиною. Не реалізовувати наступний stage у цьому чаті.

## Запит для окремого чату

```text
Реалізуй лише Stage 04 за docs/implementation-plans/pdf-compression-stages/04-ghostscript-profiles.md.
Прочитай основний план GS-v1, tracker, decisions і результат попереднього stage; звір їх із кодом.
Спочатку короткий план цього stage, потім реалізація в межах scope.
Стек: Go + open-source Ghostscript + qpdf; API внутрішній без авторизації.
Не запускай міграції. Перед кожним unit run перевір dependencies на DB interaction/data writes і запиши audit.
Запиши фактичні results у results-файл цього stage, онови tracker, залиш interfaces/artifacts і handoff.
Не реалізовуй наступні stages і не створюй нові чати автоматично.
```
