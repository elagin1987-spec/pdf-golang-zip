# Результат Stage 01: Середовище та перевірка Ghostscript

- Статус: **Completed**, із двома явно відкритими visual follow-ups для Stage 07.
- Оновлення: 2026-10-09; виконавець — поточний Codex chat.
- Контракт: GS-v1, Go 1.27.2 + **Ghostscript 10.08.0** + qpdf 12.4.2.
- Baseline: workspace містив лише implementation plans. Git repository недоступний
  (`git status`: not a git repository); commit/push не виконувалися.
- Зміна 10.09.0 → 10.08.0 явно погоджена користувачем після HTTP 404 для gs10090;
  див. [decisions](../decisions.md).

## Фактичний результат

Зібрано `pdf-engine-stage01:probe` і `pdf-engine-stage01:qa` із source-built CLI,
перевіреними SHA-256 archives, pinned base image digests і Ubuntu package snapshot
20261008T000000Z. Probe — Go executable без server/pdfworker, зовнішніх Go modules,
БД, Redis чи network clients. QA окремо додає Poppler/Pillow.

Вісім власних синтетичних PDF × qpdf lossless / GS balanced = 16 кандидатів.
Фінальний QA: **exit 0**, 14 Pass + 2 RequiresVisualFollowup. Для всіх 16 strict
qpdf check, gs txtwrite, незалежний pdftotext, page geometry, URI link і bookmark
збережені. Текст містить перевірений marker, а не порівнюються два порожні outputs.

Фактичний Poppler image inventory підтверджує 600→300 dpi для JPEG, RGB/Gray Flate
і JPX; 150 dpi залишається 150; mono/JBIG2 залишається 600 dpi. Decoded mono image
pixels тотожні. Окремий runtime-only probe без Poppler/Python: **exit 0**, 16 checks.

Детальна byte/dimension matrix, конкретні argv/settings, обмеження й artifacts:
[engine evaluation](../../../engine-evaluation.md),
[збережений lock](../../../engine-evaluation-lock.json).

## Змінені файли

- `go.mod`; `cmd/engine-probe/main.go` — standard-library probe, fixed balanced
  settings, checks/report. `go.sum` не потрібен: external modules відсутні.
- `Dockerfile`, `.dockerignore`, `.gitignore`, `Makefile` — probe/QA targets,
  digests/checksums/snapshot, non-root execution, ignored artifacts.
- `THIRD_PARTY_NOTICES.md` — included software, AGPL/BSD/Apache notices, source
  archives та compiled fonts/resources; оригінальні license texts у Docker.
- `testdata/pdf/generate.py`, `README.md`, `manifest.json` і вісім PDFs:
  `jpeg-600dpi.pdf`, `jpeg-150dpi.pdf`, `flate-rgb-600dpi.pdf`,
  `flate-rgb-150dpi.pdf`, `flate-gray-600dpi.pdf`, `mono-flate-600dpi.pdf`,
  `jbig2-600dpi.pdf`, `jpx-600dpi.pdf`.
- `scripts/check-test-dependencies.sh`, `docs/test-dependency-audit.md` — dependency
  inventory і фактичний ручний review; жодного unit run.
- `docs/engine-evaluation.md`, `docs/engine-evaluation-lock.json`, цей report,
  власний Stage 01 status/tracker, погоджений baseline у primary plan/decisions.

## Перевірки й критерії

| Критерій / команда | Фактичний результат | Доказ |
|---|---|---|
| Official sources / Docker QA build | Pass, exit 0; source SHA-256 збіглися | `artifacts/stage-01/build-pinned.log`, `build-final.log` |
| Versions без activation key | Go 1.27.2 / gs 10.08.0 / qpdf 12.4.2 / Poppler 24.02.0 | QA report versions, Dockerfile |
| `make integration RUN_ID=verified SECURITY_OPT=` | exit 0, 16 кандидатів; 14 Pass, 2 visual follow-ups | `artifacts/stage-01/verified-qa/report.json`, `verified-qa.log` |
| 300 dpi / no upscale | 1200→600 px на 2 inches; 300 px залишаються 300 | qpdf JSON, actual `pdfimages -list` у report |
| txtwrite original/output + independent extraction | Pass для всіх 16 | `.txt`, argv/stdout pdftotext у report |
| Physical boxes / URI Link / /Fit bookmark | Pass для всіх 16 | saved qpdf JSON, semantic assertions |
| Positive mono / JPX / JBIG2 | Реальний decoder/rewrite Pass на selected build | PDFs, dimensions, independent PNG extraction |
| Lossless independent render comparison | 8 Pass: 6 full pages + 2 mono image crops, 600 dpi | `.ppm`, report render assertions |
| GS decoded mono pixels | 2 Pass; source JBIG2 відповідає власному Flate pattern | extracted PNGs, decoded_image_equal, jbig2_matches_mono |
| GS mono page-crop renders | **2 differences**, причина не встановлена | source/balanced `-crop.ppm`; RequiresVisualFollowup |
| SAFER fixed paths / adjacent read + write denial | Pass, invalidfileaccess | exact gs argv/diagnostics, safer_denied=true |
| Fonts/resources/notices | Pass для потрібних core fonts; ROM resources і notices збережені | build resource log, `fonts-and-library-checksums.txt`, retained source archives |
| `make probe RUN_ID=verified SECURITY_OPT=` | exit 0; QA=false, 16 checks | `verified-probe/report.json`, `verified-probe.log` |
| Probe excludes server/worker/Poppler/Python | Pass | `probe-target-evidence.txt`, Docker target |
| Dependency audit | Reviewed; no DB/network clients | `docs/test-dependency-audit.md` |
| Unit tests | **Not run**; Stage 01 не має unit tests | audit; Makefile unit лише inventory/message |
| Migrations / DB writes | **Не запускалися** | source/dependency review; DB tooling відсутнє |

Persistent raw artifacts — під `artifacts/stage-01/`, ignored Git; synthetic only.
Image IDs, source/binary/shared-library SHA-256 збережені також у persistent source lock.
Runtime/QA installed packages та build configuration збережені в image.

## Відхилення, дефекти й обмеження

1. Ghostscript 10.08.0 замість недоступного 10.09.0 — погоджене рішення, без
   silent replacement. Інші engine/tool versions не змінювалися.
2. Snap Docker 29.8.0 відмовляє у будь-якому exec із no-new-privileges (255,
   operation not permitted), включно з /bin/sh і gs. Локальний QA виконувався
   із `SECURITY_OPT=`. Default Makefile зберігає цей прапорець; non-root,
   read-only rootfs, cap-drop ALL, no network і resource limits реально діяли.
   Підтримку прапорця повторно перевірити в Stage 06 на deployment host.
3. Fontconfig cache на read-only rootfs спричинив stderr diagnostics у першому
   запуску. `XDG_CACHE_HOME=/tmp/cache` виправляє це; final QA не мав diagnostics.
4. Mono/JBIG2 GS image pixels збережені точно, page-crop Poppler renders мають
   відмінності на краях. Не встановлено root cause і не заявлено visual fidelity
   Pass. Це обов'язковий follow-up/release gate для Stage 07.
5. Деякі GS candidates більші за original: це виміряний результат, а не помилка
   probe. Stage 05 має вибирати smallest valid qpdf/original candidate.
6. Fixtures покривають JPX RGB lossless і JBIG2 MMR generic region без globals /
   symbol dictionaries. Широку codec/quality acceptance залишено Stage 04/07.
7. Generic semantic validator, compact profile, policy, process groups, HTTP API,
   Windows/runtime deploy, quality/load acceptance не реалізовані в цьому stage.

Початкові failed diagnostics залишені як diagnostics, а не приймальні Pass.
Не створено Go resizer, PDF CTM lexer, JSON update generator або server binaries.

## Передача Stage 02

Stage 01 локальні criteria виконані; Stage 02 може починатися після перевірки
цього report із actual files. Продуктовий сервіс ще не готовий.

- Tool executables: `/opt/pdf-tools/bin/gs` 10.08.0,
  `/opt/pdf-tools/bin/qpdf` 12.4.2; PATH і LD_LIBRARY_PATH задає Dockerfile.
- CLI: `engine-probe -fixtures /fixtures -out <new-directory> -qa=true|false`.
  Source/QA fixtures і JSON report schema — diagnostic interfaces, не production
  adapters. Fixed profile settings лишаються Stage 01 evidence; Stage 04 створює
  production profile code, не копіює весь probe у service.
- Source of truth для fixtures: saved source PDFs + manifest SHA-256. Не запускати
  regeneration без потреби: Pillow versions можуть змінити encoded bytes.
- Read-only container потребує writable tmpfs для Fontconfig cache, крім job dir.
  Job evidence містить SAFER permits лише для конкретних fixed input/output paths.
- Stage 02 реалізує тільки adapters/process runner за своїм stage-file, з audit
  перед кожним unit run. Передати Stage 07 mono crop discrepancy і Stage 06
  no-new-privileges host limitation; не починати ці stages зараз.
