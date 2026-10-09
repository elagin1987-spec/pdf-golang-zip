# Покрокова реалізація PDF compression service

Основний контракт: [план](../2026-10-09-pdf-compression-service.md). Рушії та компроміси: [порівняння](../pdf-compression-engine-options.md).

Кожен stage — окреме завдання для нового чату в цій папці проєкту. Stage-файл містить готовий запит; скопіювати його в потрібний чат. Самі чати цим планом не створюються. Логічна послідовність: 01 → 02 → 03 → 04 → 05 → 06 → 07 → 08.

## Tracker — GS-v1

| Stage | Завдання | Залежить від | Статус | Фактичний результат |
|---|---|---|---|---|
| 01 | [Середовище та перевірка Ghostscript](01-environment-and-probe.md) | — | Completed | [Звіт](results/01-environment-and-probe.md) |
| 02 | [Adapters Ghostscript/qpdf та процеси](02-tool-adapters.md) | 01 | Completed | [Звіт](results/02-tool-adapters.md) |
| 03 | [Аналіз PDF та політика MVP](03-analysis-and-policy.md) | 02 | Completed | [Звіт](results/03-analysis-and-policy.md) |
| 04 | [Профілі стиснення Ghostscript](04-ghostscript-profiles.md) | 03 | Completed | [Звіт](results/04-ghostscript-profiles.md) |
| 05 | [Конвеєр та перевірка кандидатів](05-compression-pipeline.md) | 04 | Completed | [Звіт](results/05-compression-pipeline.md) |
| 06 | [HTTP API та робочий контейнер](06-http-and-worker.md) | 05 | Completed | [Звіт](results/06-http-and-worker.md) |
| 07 | [Якість PDF та навантаження](07-quality-and-load.md) | 06 | Completed | [Звіт](results/07-quality-and-load.md) |
| 08 | [Фінальна документація та передача](08-handoff.md) | 07 | Completed | [Звіт](results/08-handoff.md) |

Stages 01–08 завершено для локального GS-v1 handoff; фактичні докази наведено у звітах.
Пакет передачі — `artifacts/stage-08/`; цільовий host acceptance та межі — у `docs/operations.md`.

## Правила роботи в одному чаті

1. Прочитати основний контракт, stage-файл, decisions і report попереднього stage. Звірити reported interfaces/artifacts із кодом; не покладатися лише на текст звіту.
2. Назвати короткий план поточного stage. Виконувати тільки його scope; необхідні мінімальні правки попереднього етапу пояснити у звіті. Не переписувати весь pipeline і не починати наступний stage.
3. Міграції заборонені. Перед кожним unit run — перевірка тестів/imports/helpers/init/TestMain/config на DB interaction/data writes із записом у audit. Unit tests без БД і зовнішніх сервісів.
4. Статуси: Not started → In progress → Completed; Blocked допускається з конкретним blocker і потрібною дією. Completed потребує доказу кожного локального acceptance criterion; загальна готовність сервісу визначається тільки після Stage 08.
5. У кінці заповнити results-файл: зміни, commands/results/artifacts, залежності тестів, рішення/відхилення, дефекти, interfaces й handoff. Оновити лише власний рядок tracker. Не позначати невиконаний тест Pass.
6. Записати commit hash лише якщо commit фактично є. Цей workflow не вимагає автоматичного commit/push/deploy. У незакоміченій роботі перелічити точні файли та baseline, з якої продовжувати.
7. Наступний чат починає наступний stage тільки після Completed попереднього. Якщо результати/code розійшлися, виправити розбіжність і report до продовження.

## Рішення та зміна engine

[decisions.md](decisions.md) — журнал material decisions. Користувач явно обрав **Go + open-source Ghostscript + qpdf**, редакція GS-v1. qpdf робить preflight/lossless/structural checks; Ghostscript — balanced/compact rewrite. Custom Go codecs/CTM lexer більше не плануються.

При явному виборі іншого engine оновити основний контракт, version baseline, codec policy, зачеплені stage scopes/tests і відповідні reports. Вже завершений stage із доказами старого engine не автоматично Completed для нового; у tracker зазначити In progress і конкретну повторну перевірку. Не починати два engines паралельно без такого завдання.

## Де зберігаються докази

Results-файли містять facts і посилання. Великі PDF/render artifacts — під `artifacts/stage-NN/` із gitignore, якщо їх створено в реалізації; source fixtures — `testdata/pdf/` з дозволеним походженням. Persistent artifacts не містять реальних приватних документів. Жодних placeholder timestamps або fabricated performance numbers.
