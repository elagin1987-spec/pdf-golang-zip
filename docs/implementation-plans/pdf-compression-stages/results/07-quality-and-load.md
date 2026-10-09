# Результат Stage 07: Якість PDF та навантаження

- Статус: **Completed**.
- Оновлено: **2026-10-09**, поточний чат реалізації, GS-v1.
- План: [07-execution-plan.md](../07-execution-plan.md), створено до реалізації;
  користувач прямо доручив створити план і виконати його.
- Baseline: Stage 06 interfaces/code/config/Docker звірено; Go 1.27.2,
  Ghostscript 10.08.0, qpdf 12.4.2. Production код/профілі/caps не змінено.
- Git: `.git` не є repository; commit/push не створено.

## Фактичний результат

Незалежна quality acceptance: **32 fixtures × 3 profiles = 96 Pass**;
69 успішних PDF + 27 special-type refusals. Реальні integration/HTTP tests пройшли,
Poppler text/order/geometry/renders і zbar QR/Code128 пройшли. Незалежні pypdf
geometry/URI/GoTo/bookmarks — **69/69**. Mono returned — **12/12** exact decoded
pixels, raw GS — **8/8**; CCITT/JBIG2/Flate cross-codec hashes однакові.

Lossless — **23/23** exact render sets. Balanced/compact appearance перевірено
numeric screening + relevant manual renders; кирилиця, 6pt text, вектори, masks,
inline/nested Forms/shared placement. UserUnit/rotation fixture коректно fallback:
`ghostscript_features_changed`, exact qpdf output. CMYK/ICC і unsupported ASCIIHex
мають явні warnings. Codec matrix має actual decoder/selected-engine evidence.

Load: **12 positive batch cases + 4 refusal cases Pass**. 1/2 workers, до
**500 pages / exactly 50 MiB**, три profiles на максимальному PDF. Max sampled
cgroup memory **944.8 MiB**, aggregate job logical disk **535.2 MiB**, caps збережені.
Max-load balanced batch **8.77 s / 13.49 s** (1/2 jobs), compact **7.92 s / 13.33 s**.
Busy/cancel/cleanup/timeout/shutdown і 501-page/50MiB+1 boundaries реально перевірено.
Dedup on/off — 3 alternating trials для duplicate/unique corpus; enabled залишено.

Подробиці: [quality-report.md](../../../quality-report.md),
[benchmark-report.md](../../../benchmark-report.md).

## Змінені файли та interfaces

- `testdata/pdf/generate_quality.py`, `quality-manifest.json`, 24 new PDFs,
  `dejavu-font-license.txt`, fixture README — owned synthetic provenance/checksums;
  existing 8 Stage 01 source PDFs і їхній manifest не перегенеровано.
- `tests/integration/compress_test.go` — fixture hash/strict real pipeline/semantic snapshot
  matrix для всіх profiles/refusals/fallback; не дозволяє missing matrix fixtures.
- `tests/e2e/api_test.go` — 96-case real HTTP matrix і active max-load busy/cancel
  для workers 1/2 з observed/reaped worker/GS IDs.
- `scripts/quality_load.py` — independent Poppler/zbar quality та streaming local
  HTTP benchmark; sampled cgroup/disk, strict checks, persistent evidence, no DB/network.
- `scripts/check_quality_features.py` — independent pypdf geometry/link/bookmark snapshots
  без production helper; read-only saved input/selected outputs.
- `scripts/quality-profiles/main.go` — QA-only actual `ghostscript.BuildArgs` JSON export.
- `Dockerfile` — quality-build/quality-test targets, zbar/Poppler/Python/Go тільки QA;
  production target не змінювався. `Makefile` — quality-fixtures/build/check/load/tests.
- `THIRD_PARTY_NOTICES.md` — fixture subset font/zbar attribution.
- `docs/{quality-report,benchmark-report,test-dependency-audit}.md`,
  stage plan/execution-plan/results, `decisions.md`, тільки Stage 07 tracker row.

Public HTTP API/headers/warnings unchanged; QA export не є endpoint. Load files
генеруються під artifacts (gitignored), не 50MiB source binary у repository.
`QUALITY_PYTHON` має Pillow/reportlab/pypdf, installed DejaVu font; host bundled
Python використано фактично. `QUALITY_RUN` дає fresh timestamped QA folder;
QA не перезаписує existing acceptance output. Source fixtures SHA — source of truth.

## Перевірки та докази

Усі persistent докази: [artifacts/stage-07](../../../../artifacts/stage-07/).
Raw input/selected/GS/lossless PDFs, qpdf JSON, pdfinfo/pdffonts, Poppler text/renders,
contact sheets — `quality-accepted/<fixture>/<profile>/`; commands мають actual exit/stderr.

| Перевірка | Фактичний результат | Артефакт |
|---|---|---|
| Unit dependencies audit | Pass перед U1; stdlib/local modules, private temp/fakes, no DB/init/TestMain/remote endpoints | docs/test-dependency-audit.md S07-U1/R1–R3/L1, dependency-inventory/review |
| Unit race U1 | Pass, exit 0; 8 internal packages + cmd/pdfworker | unit-race.log |
| Real integration + e2e R3 | Pass, exit 0, no Skip; full matrix/busy/cancel/timeout/shutdown | real-tests-accepted.log |
| Independent quality R3 | Pass, exit 0, 96 rows; raw GS mono/cross-codec exact | quality-accepted.log, quality-accepted/quality.json, mono-codec-crosscheck.json |
| Independent features | Pass, exit 0, 69 snapshots exact/tolerance | independent-features.log, quality-accepted/independent-features.json |
| Manual review | Pass relevant full renders/contact sheets; accepted PNGs equal reviewed PNGs | manual-review.md, manual-review-equivalence.json |
| Load + dedup | Pass, exit 0; 16 rows, 12 GS microbenchmark calls | load.log, benchmark.json, dedup.json, load-commands.json |
| Own container cleanup | Pass, no pdf-stage07 containers remain | container-cleanup.txt |
| Binary/image/source identity | Actual values recorded | image-id.txt, binary-sha256.txt, source-sha256.txt |

### Фактичні команди

Використано `/snap/docker/current/bin/docker`, нижче `docker` означає саме його.
Work root: `/home/alex/Desktop/pdf-golang-zip`. Перед кожним unit run — inventory/manual
review/audit entry. Не запускали міграції, БД/Redis/remote APIs або processing private PDFs.

```sh
# Fixture creation, using bundled Python deps (host-side QA only)
/home/alex/.cache/codex-runtimes/codex-primary-runtime/dependencies/python/bin/python3 testdata/pdf/generate_quality.py
./scripts/check-test-dependencies.sh

# U1, before execution audit appended
# Mount source readonly; test binaries/private fake tools require exec temp only in harness.
docker run --rm --init --user 1000:1000 --network none --read-only --cap-drop ALL \
  --tmpfs /tmp:rw,exec,size=1g -e GOCACHE=/tmp/go-cache -e GOPATH=/tmp/go-path \
  -e GOFLAGS=-buildvcs=false -v /home/alex/Desktop/pdf-golang-zip:/src:ro -w /src \
  golang:1.27.2-bookworm go test -race -count=1 ./internal/... ./cmd/pdfworker

docker build --target quality-test -t pdf-compression:stage07-quality .

# R3 independent Poppler/mono/zbar acceptance
docker run --rm --init --name pdf-stage07-quality --user 1000:1000 --network none \
  --read-only --cap-drop ALL --memory 2g --cpus 2 --pids-limit 128 \
  --tmpfs /tmp:rw,noexec,nosuid,size=256m \
  -v /home/alex/Desktop/pdf-golang-zip:/src:ro \
  -v /home/alex/Desktop/pdf-golang-zip/artifacts/stage-07:/evidence \
  pdf-compression:stage07-quality --mode quality --out /evidence/quality-accepted

# R3 real integration/API and active heavy processing cancellation
docker run --rm --init --name pdf-stage07-tests --user 1000:1000 --network none \
  --read-only --cap-drop ALL --memory 2g --cpus 2 --pids-limit 128 \
  --tmpfs /tmp:rw,exec,nosuid,size=1g --entrypoint go \
  -e PDF_LOAD_FIXTURES=/evidence/load-fixtures \
  -v /home/alex/Desktop/pdf-golang-zip:/src:ro \
  -v /home/alex/Desktop/pdf-golang-zip/artifacts/stage-07:/evidence:ro \
  pdf-compression:stage07-quality test -v -count=1 ./tests/integration ./tests/e2e

# L1, after QA/tests completed; original production resource boundaries
docker run --rm --init --name pdf-stage07-load --user 1000:1000 --network none \
  --read-only --cap-drop ALL --memory 2g --cpus 2 --pids-limit 128 \
  --tmpfs /tmp:rw,noexec,nosuid,size=256m \
  --tmpfs /work/jobs/runtime:rw,noexec,nosuid,size=1200m,mode=1777 \
  -v /home/alex/Desktop/pdf-golang-zip:/src:ro \
  -v /home/alex/Desktop/pdf-golang-zip/artifacts/stage-07:/evidence \
  pdf-compression:stage07-quality --mode load --out /evidence

/home/alex/.cache/codex-runtimes/codex-primary-runtime/dependencies/python/bin/python3 \
  scripts/check_quality_features.py artifacts/stage-07/quality-accepted
```

Artifact-only fresh folder потрібний для нового QA run; existing acceptance folder
після final hardening не перезаписується. Вимірювання в accepted evidence залишаються
від конкретних вказаних команд. Go source/scripts/fixtures mounted read-only `/src`
перекривають COPY snapshot тестового образу; source-sha256 фіксує фінальний стан.

QA image: `sha256:72dda00f7a35239d248ff12ea6ca86a6544a4111371124b3510d454d49bad6fb`.
server/worker/tool hashes — binary-sha256.txt; gs/qpdf версії збігаються зі Stage 06.
QA Poppler version з actual stderr, збереженого в commands; measurements не вигадані.

## Дефекти, відхилення та обмеження

1. Перші negative fixtures мали direct form field; qpdf exit 3. Generator виправлено
   на indirect object. Не було доведено product defect.
2. CCITT polarity була inverted проти mono Flate; BlackIs1 виправлено і independent
   decoded equality повторено. R2 мав 96 Pass rows, але overall Fail — не acceptance.
3. Load fixture padding after EOF ламав strict qpdf preflight; перенесено before xref
   з correct startxref. Попередній cancel run Fail з invalid_pdf збережений.
4. Indexed compact MAE 6.451/255: recorded manual acceptance/screening bound 8 лише
   для high-contrast fixture; жодного production cap/profile change.
5. UID root у formatter container не мав workspace write permission; formatting
   виконано з UID власника 1000. Це не test acceptance attempt.
6. Snap Docker no-new-privileges deviation успадковано зі Stage 06; option на сумісному
   host **Not run**. Runtime default залишено; security acceptance не вигадується.
7. 20ms peaks sampled, API case по одному trial, dedup по три; universal worst-case,
   production hardware/real scanner та JBIG2 symbols/globals **Not run**.
8. Final QA tooling hardening/version bookkeeping після accepted checks не змінює
   production/test algorithms/dependency closure; no extra unit run required.

Всі локальні Stage 07 criteria мають Pass або погоджений bounded fallback; unresolved
failed local requirements немає. Scope Stage 08 не реалізовувався.

## Передача

Stage 07 Completed; наступний **Stage 08 — handoff** може починатися зі звірки цього
report, quality/benchmark reports і actual artifacts. Public API та production settings
unchanged. Не обіцяти byte identity GS streams/IDs/codecs, universal decoder corpus,
statistical throughput або default no-new-privileges acceptance цього host.
Stage 08 потрібний для загальної готовності/остаточної документації, не виконувався тут.
