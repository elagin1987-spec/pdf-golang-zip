# Журнал рішень

## 2026-10-09 — підтверджено користувачем

Go/Linux/Docker; локальні PDF tools допустимі. Лише HTTP API. Balanced 300 dpi, lossless, compact 150 dpi. Звичайні PDF і скани; special types відхиляти. Один сервер зі стартовими лімітами. Код закритий. API внутрішній, без авторизації, акцент на функціональності. Окремі stages для окремих чатів і фіксація фактичних результатів.

## 2026-10-09 — інженерний baseline

Go + qpdf + x/image без Ghostscript у базовій реалізації; Poppler лише QA. Це вибір автора плану для стека без комерційної PDF-ліцензії/AGPL, а не пряма відмова користувача від Ghostscript чи встановлена заборона internal usage. Ghostscript/commercial і pikepdf/Pillow залишаються описаними альтернативами; переходу на них користувач ще не доручав.

## 2026-10-09 — уточнення ліцензійного пояснення

Закритий код не є самостійною підставою категорично забороняти Ghostscript. Artifex публікує позицію щодо server-based integration; фактична оцінка AGPL залежить від integration/modifications/distribution. Окремий process або internal network не дають автоматичної відповіді. Див. порівняння engine options і первинні посилання.

## Формат наступного запису

Дата; stage/chat; конкретний факт або рішення; причина й докази; зачеплені files/contracts; рішення користувача або явно назване інженерне припущення; які попередні checks треба повторити. Не змінювати історію попередніх фактичних звітів мовчки.

## 2026-10-09 — явний вибір користувача: GS-v1

Користувач доручив взяти першим варіантом реалізацію з Ghostscript після пояснення open-source варіанта без activation key. Основний стек тепер Go + open-source Ghostscript + qpdf, Poppler лише QA. Ghostscript виконує balanced/compact downsampling; qpdf — policy/lossless/structural checks. Закритий код і внутрішній API без авторизації залишаються. Рішення про engine не є юридичним висновком щодо AGPL.

Попередній Go-image-editor baseline замінено: вилучено x/image, власні codecs/CTM lexer і update-from-json. Семантична/text/visual validation заміняє вимогу тотожних content-stream hashes після GS. Кількість optimized-images більше не заявляється; reports показують engine і inventory counts. Stage 02/03/04 та їхні порожні results перейменовано; всі вісім stages Not started, попередніх execution results немає.

## 2026-10-09 — Stage 01: погоджений release baseline

Офіційний release gs10090 повертає HTTP 404; сайт Artifex називає latest release 10.08.0. Користувач явно погодив використання Ghostscript 10.08.0 замість 10.09.0. Основний контракт оновлено; Go 1.27.2 і qpdf 12.4.2 залишаються. Source SHA-256 і image digests зафіксовані в Dockerfile. Попередніх completed checks немає; наступні stages використовують цей baseline.

## 2026-10-09 — Stage 06: runtime boundaries та локальний Docker

Mandatory qpdf lossless write/validation failure тепер повертає помилку, а не прихований
success/original: потрібна мінімальна правка Stage 05 для HTTP 500/storage contract.
Missing tool і missing job file розрізняються (503 і 500). Caps лишаються GS-v1.
Worker успадковує одну group для gs/qpdf, parent cancel прибирає всю group; докази обох CLI
та cleanup записані у Stage 06 results. Watcher polling допускає короткий overshoot;
Compose tmpfs задає сумарний storage bound.

Локальний snap Docker не запускає init із no-new-privileges (operation not permitted).
У Compose додано явний NO_NEW_PRIVILEGES toggle, default true. Локальні checks використовують
false; інших protections не послаблено. Це environment compatibility deviation,
не підтвердження роботи no-new-privileges на цьому host. Stage 06 functional API completed;
перевірка default security option на сумісному host лишається operational follow-up.

## 2026-10-09 — Stage 07: independent QA and bounded benchmark

Production profiles/caps unchanged. Poppler renders/text and independent pypdf physical
geometry/URI/GoTo/bookmarks plus zbar symbols complement runtime GS/qpdf checks.
Lossless pixels remain exact; mono decoded equality includes raw GS even when smallest
selection returns qpdf/original. UserUnit/Rotate fixture triggers existing truthful
`ghostscript_features_changed` fallback; supported plain shared placements do pass GS.

QA screening MAE 6/255 is an engineering heuristic, not a contract-wide fidelity claim.
Compact Indexed high-contrast fixture has MAE 6.451/255; reviewed edge smoothing/chroma
changes are acceptable at 150dpi. Its explicit fixture-specific screening bound is 8/255;
all exact-lossless and mono requirements remain. This calibration is recorded in
quality manifest/report and manual review; no resource/time/profile flags were relaxed.

Fixture defects found and fixed: indirect AcroForm field, CCITT BlackIs1 polarity,
50MiB padding before xref. Failed attempts remain evidence and are not Pass.
1/2 workers, 500 pages and exactly 50MiB pass within original 2GiB/2CPU/120s/512MiB-per-job
bounds. Dedup enabled remains: three alternating measurements show substantial size
benefit on duplicates without an observed time penalty on this synthetic 64-image corpus.
Results are a local bounded benchmark, not a universal throughput guarantee.

## 2026-10-09 — Stage 08: local handoff and immutable rollback

User authorized planning and executing the next stage. All 125 Stage 07 source hashes
matched before editorial changes; final runtime server/worker/gs/qpdf hashes equal
accepted Stage 07 binaries. Existing runtime image is saved and reloaded, not rebuilt
for doc-only changes. Its installed notices remain the Stage 06 snapshot; final notices
are also in source package, underlying licenses/source archives are unchanged.

Compose PDF_IMAGE now supports a verified local image ID for rollback; default remains
the accepted stage06 tag. No behavior/profile/cap changes. Local Compose 2.3.3 rejects
up --pull never, so instructions inspect the loaded local ID and use up --no-build.
Failed first invocation is retained, not marked Pass. Local smoke follows the existing
snap NO_NEW_PRIVILEGES=false exception; target-host default protection still Not run.

Local handoff is complete with explicit bounded codec/visual/performance evidence;
production host/deploy and rollback between different releases are not claimed.
No unit rerun: Go/tests unchanged, relevant prior passed evidence retained. No DB,
migrations, commit, push or deploy. Stage 08 is the final planned stage.

Stage 08 actual container inspection found Compose 2.3.3 ignored cpus:2 (NanoCpus,
CpuQuota/CpuPeriod all zero despite resolved cpus). Replaced shorthand with explicit
cpu_period=100000/cpu_quota=200000, preserving 2 CPU contract; final smoke/inspect
repeated. Previous inspect retained. Stage 06 historical Compose resource claim
is corrected by this note: CPU isolation was not proven there. Stage 07 benchmark
used docker run --cpus 2, so its accepted CPU bounds remain valid. No Go/unit changes.
