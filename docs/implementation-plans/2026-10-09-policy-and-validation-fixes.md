# План виправлення policy та обробки помилок перевірки PDF

Дата: 2026-10-09. Статус: **Completed**, після авторизації користувача.
Результати й фактичні обмеження: [звіт](../policy-validation-fixes-report.md).

## Мета і межі

Виправити три розбіжності з GS-v1: пропуск вкладених заборонених дій,
ігнорування помилок читання наявного XMP та приховування інфраструктурних
помилок qpdf під час перевірки GS-кандидатів.

Профілі стиснення, версії інструментів, ресурсні ліміти й успішний API-контракт
залишаються поточними. Міграції та взаємодія з БД заборонені.
Перевірки на цільовому сервері та реальних документах є окремим наступним кроком.

## 1. Вкладені заборонені дії

Файли: `internal/pdf/policy.go`, `internal/pdf/policy_test.go`,
`internal/pdf/analyzer.go`, `internal/pdf/analyzer_test.go` за потреби
для передачі помилок обмеженого обходу.

- Винести наявні перевірки заборонених дій у спільний helper і застосувати
  їх до вкладених словників та масивів структурних JSON-об'єктів, включно
  зі словниками потоків. Перевіряти `/AA`, `/Next`, `/OpenAction`, `/A`.
- Перевіряти прямі вкладені словники; непрямі об'єкти вже є в qpdf JSON,
  тому не обходити граф посилань повторно без потреби. Якщо helper розв'язує
  посилання, використовувати visited-set і захист від циклів.
- Не аналізувати текстові значення або байти PDF streams як дії:
  рядок з текстом `/JavaScript` сам по собі не є причиною відмови.
- Зберегти дозволені прості URI/GoTo links і наявні refusal codes.
  Заборонений JavaScript/Launch повертає `422 special_type_javascript`.
- Обхід обмежити чинними depth/node caps; перевищення завершує перевірку
  з `422 document_limit`, а не повертає Allowed після часткового аналізу.
- Тести: direct/indirect JavaScript у catalog/page `/AA`, link `/AA`,
  `/Next` як dictionary/array, вкладений Launch; clean URI/GoTo;
  текст зі схожими словами; межі обходу та цикли, якщо посилання розв'язуються.

Критерій: заборонена вкладена дія відхиляється до генерації lossless/GS;
чисті PDF та дозволені links проходять.

## 2. Помилки читання XMP

Файли: `internal/pdf/analyzer.go`, `internal/pdf/analyzer_test.go`,
`internal/compression/service_test.go`, `cmd/pdfworker/main_test.go`.

- Відсутній `/Metadata` лишається нормальним випадком.
- Якщо `/Metadata` є, але його reference некоректний або stream неможливо
  прочитати, припиняти аналіз з помилкою; не передавати порожній XMP у policy.
- Зберігати причину через `%w` та `context` cancellation/deadline.
- Зберегти чинну відмову для XMP понад 4 MiB і для прочитаної декларації PDF/A.
  Звичайна помилка читання не повинна неправдиво стверджувати, що це PDF/A.
- API: missing/unlaunchable tool — `503 tool_unavailable`, інші внутрішні
  помилки читання — `500 processing_failed`, timeout — `504 processing_timeout`.
  Сирі шляхи, stderr і метадані не потрапляють у відповідь або логи.
- Тести: немає Metadata; valid non-PDF/A; declared PDF/A; stream over cap;
  missing tool; generic read failure; malformed reference; canceled context.
  На рівні Service довести, що помилка зупиняє всі rewrite operations.

Критерій: успішна відповідь неможлива після невиконаної перевірки наявного XMP.

## 3. Помилки qpdf при перевірці GS-кандидатів

Файли: `internal/compression/validation.go`, `internal/compression/service.go`,
їхні `_test.go`, `internal/qpdf/client.go`/`json.go` та їхні тести лише якщо
потрібне уточнення типізації, `cmd/pdfworker/main_test.go`.

- Розрізняти непридатний кандидат та недоступну перевірку:
  qpdf check exit 2/3, невалідна структура кандидата або зміна geometry/links/text
  відхиляють GS-кандидат із чинним warning і дозволяють valid fallback.
- Помилка запуску qpdf, збій процесу без результату перевірки, storage error,
  перевищення загальних JSON/object limits або cancellation передаються як error.
  Для аналізу JSON використати явні типи/sentinel errors, якщо їх бракує;
  не класифікувати помилки за текстом stderr.
- Зберігати error chain. Missing tool — 503; внутрішній збій — 500;
  document limits — 422; processing timeout — 504.
- У Service обов'язково передавати error з ValidateCandidate для обох
  `gs-candidate.pdf` і `gs-structural.pdf`: зараз `val_err`/`val_err2` ігноруються.
- Уточнити обробку WriteLossless для optional GS structural candidate:
  неможливість виконати required tool/storage checks не приховується успіхом;
  документна непридатність optional candidate дозволяє інший valid candidate.
- Тести ін'єктують missing tool, process failure, storage failure, limits,
  cancellation окремо на check/analyze та на обох GS-кандидатах.
  Контрольні тести зберігають fallback для damaged candidate,
  geometry/text mismatch і недоступної додаткової text validation.

Критерій: infrastructure failure не повертає 200; відхилений GS-кандидат
не віддається; передбачені планом semantic/text fallbacks працюють.

## 4. Перевірки та оновлення доказів

1. Спочатку додати регресійні сценарії, що відтворюють три проблеми.
2. Перед КОЖНИМ unit run перевірити imports/dependency closure/helpers,
   init/TestMain, конфігурацію й операції з файлами/процесами. Зафіксувати
   аудит у `docs/test-dependency-audit.md`. Inventory script не замінює огляд.
   Fake runners, synthetic JSON та приватні `t.TempDir()`; жодної БД.
3. Запустити targeted tests для pdf/qpdf/compression/pdfworker у контейнері
   із pinned Go, network none, read-only source та приватними cache/tmp.
   Після правок провести один повний unit run із race detector для
   `./internal/... ./cmd/pdfworker`, повторно виконавши аудит.
4. Додати owned synthetic PDFs для вкладених дій у окремий regression corpus
   з provenance/SHA; довести real HTTP 422, відсутність PDF response і cleanup.
   Infrastructure failures перевіряти ін'єкціями, не змінюючи системні tools.
5. Провести на оновлених binaries чинну real integration/e2e matrix та
   незалежну quality matrix: policy змінює допуск документів.
   Попередні 96/16 результати не видавати за доказ нового build.
   Повний load benchmark повторювати лише за зміни caps/pipeline cost
   або появи регресії; targeted bounded-traversal check виконати обов'язково.
6. Зберегти нові logs/source/binary hashes в окремій папці artifacts;
   старі accepted/failed evidence не перезаписувати. Оновити operations,
   acceptance evidence та короткий результат виправлень.
   Якщо готується оновлений handoff, створити нові runtime/source/evidence
   архіви й SHA256SUMS; старий Stage 08 пакет залишити історичним.

## Приймання

- Усі три регресії відтворені тестами та виправлені.
- Clean corpus і три профілі працюють; output valid та не більший за input.
- HTTP-коди, fallback warnings, cancellation і cleanup відповідають контракту.
- Перевірки завершені без БД, міграцій та зовнішньої обробки документів.
- Звіт відокремлює нові результати від історичних та невиконаних host checks.

Порядок реалізації: вкладені дії → XMP → qpdf errors → спільна перевірка/звіт.
