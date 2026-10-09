# Виправлення policy/XMP/qpdf — 2026-10-09

Статус: **Completed**, локальна реалізація і перевірки за
[планом](implementation-plans/2026-10-09-policy-and-validation-fixes.md).

## Що виправлено

1. Policy перевіряє вкладені словники/масиви дій, включно з `/AA`, `/Next`,
   прямими та непрямими JavaScript/Launch. Службові поля qpdf JSON пропускаються;
   текстові значення не трактуються як дії. Кожний непрямий об'єкт перевіряється
   без повторного обходу reference graph. Depth 32 та 100000 container nodes
   обмежують перевірку; неповний обхід повертає 422.
2. Наявний `/Metadata` обов'язково читається. Некоректний reference або read
   failure зупиняє обробку, зберігаючи error chain. Відсутній XMP дозволений;
   declared PDF/A та перевищення чинного XMP cap зберігають попередні відмови.
3. Перевірка обох GS-кандидатів передає infrastructure errors у Service/API.
   Missing/unlaunchable qpdf — 503; internal process/storage failure — 500;
   JSON/object limits — 422; processing timeout — 504. Document-invalid
   candidate, geometry/text mismatch та optional text-check failure зберігають
   fallback warnings. Optional qpdf finalization також не приховує
   infrastructure failure успішною відповіддю.

Профілі, pinned tools, caps, endpoint/headers та правила вибору найменшого valid
PDF не змінено. Додано typed malformed-JSON error та регресійні перевірки.

Production files: `internal/pdf/{policy,analyzer}.go`,
`internal/compression/{service,validation}.go`, `internal/qpdf/{client,json}.go`.
Тести: нові regression suites у pdf/qpdf/compression/e2e, worker error-boundary
cases; owned corpus `testdata/pdf/regressions/` (8 PDFs + provenance/SHA).
Оновлено audit, operations, acceptance, README, план і цей звіт.

## Фактичні перевірки

Evidence root: `artifacts/policy-validation-fixes/`. Audit перед кожним unit run
зафіксований у `docs/test-dependency-audit.md`; лише stdlib/local Go modules,
fake runners/private temp, без БД, міграцій або зовнішніх документів.

| Перевірка | Результат | Evidence |
|---|---|---|
| Відтворення трьох початкових проблем | Expected Fail на попередньому коді | regression-before-retry.log |
| Full unit/race | Pass: 8 internal packages + pdfworker, exit 0 | unit-race-accepted.log |
| Real integration/e2e | Pass, exit 0, no Skip; існуюча 96-case HTTP matrix, max-load busy/cancel/cleanup | real-tests-accepted.log |
| Новий HTTP regression corpus | 24/24 Pass: 18 refusals + 6 URI/GoTo controls, 3 profiles; qpdf checks і cleanup | real-tests-accepted.log; regressions/manifest.json |
| Independent quality | 96/96 Pass: 69 PDFs + 27 refusals | quality-accepted/quality.json; quality-accepted.log |
| Independent pypdf features | 69/69 Pass: geometry/URI/GoTo/bookmarks | independent-features.log; quality-accepted/independent-features.json |
| Visual equivalence | 109/109 page PNGs byte-identical to prior accepted/manual-reviewed corpus; no new manual acceptance claim | render-equivalence.json |
| Fresh load/dedup | 16/16 Pass: 12 positive batches + 4 boundary refusals; 12 dedup tool calls | load-accepted-retry/{benchmark,dedup}.json; load-accepted-retry.log |
| Fixture/source/binary identity | Recorded; 5 load-fixture hashes match; source unchanged during accepted tests | load-fixture-verification.json; source-sha256.json; runtime-binaries.sha256; images.jsonl |
| Own QA containers after checks | None remain | remaining-qa-containers.txt |

Load: 1/2 workers, до 500 pages / exactly 50 MiB. Peak sampled memory
**935.4 MiB**, aggregate job logical disk **502.4 MiB**, single-job peak
**272.7 MiB**. Max-PDF balanced batches: **8.10 / 11.06 s** (1/2 workers);
compact: **7.45 / 11.13 s**. Це локальні вимірювання, не throughput guarantee.
Стандартні 2 GiB/2 CPU/128 PID/job 512 MiB caps збережені.

Unit: pinned Go image, init, network none, read-only source/private exec tmpfs.
Real tests: updated QA image, same isolation, loopback only, owned fixtures readonly.
Quality/load: current server/worker plus accepted pinned QA toolchain;
source/scripts mounted readonly at `/src`, only new evidence/job tmpfs writable.
Independent pypdf uses bundled local Python and reads saved PDFs only.

## Збірка й межі

Новий локальний runtime: `pdf-compression:policy-validation`:
`sha256:61f69443126e310ed00a85eaa1d5e1bedfb65aea001c117c20e51989f5083b40`.
QA: `pdf-compression:policy-validation-quality`:
`sha256:bd358717e41c379a98f3e6c7ea6d2688d635b6355ecfc858f7066eb0867ea950`.
QA reuses the accepted pinned Stage 07 tool image and copies both freshly built
runtime binaries; recipe saved as `artifacts/policy-validation-fixes/Dockerfile.quality`.
Exact image IDs and all four binary SHA-256 are recorded. No tool substitution.

Старий Stage 08 пакет залишається історичною версією до виправлень.
Оновлений source/runtime/evidence архів не готувався; для поточного запуску
використати новий runtime image або збірку поточного source.
Deployment, target host protections і реальні scanner documents не перевірялися.
Local Snap Docker потребує попереднього no-new-privileges exception; production
Compose default не змінено. Universal fidelity/JBIG2 globals не заявляються.

Невдалі спроби збережені: no-new-privileges блокував Go до тестів; unit container
без init не reap-ив orphan descendants; перший real run знайшов qpdf header
regression (виправлено й додано control); перший load run не мав fixture mount
(повторено зі SHA-verified readonly corpus). Жодна з них не позначена Pass.

Міграції/БД/production documents/deploy/commit не виконувалися. Git repository
у workspace відсутній. Після accepted runs production Go не змінювався.
