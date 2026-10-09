# Результат Stage 06: HTTP API та робочий контейнер

- Статус: **Completed**.
- Оновлено: 2026-10-09; поточний chat реалізації.
- Контракт: GS-v1, Go 1.27.2 + Ghostscript 10.08.0 + qpdf 12.4.2.
- Baseline: Stage 05 interfaces звірено з кодом; його unit regressions і real pipeline пройшли.
- Git: `.git` порожній, `git status` повідомляє not a git repository; commit не створено.

## Реалізований результат

`POST /v1/compress` приймає streaming multipart без credentials. Два slots захоплюються
до читання upload; зайнятість повертає 503/Retry-After 1. Поля в довільному порядку,
повтори/невідомі поля/порожні файли/невалідний profile відхиляються. Обмежуються file/body/text
bytes; filename/MIME не використовуються як доказ формату або шлях. Початкові 1024 bytes
перевіряються на `%PDF-`, остаточний preflight робить qpdf.

Окремий `pdfworker` виконує pipeline; gs/qpdf успадковують його process group. Timeout починається
після upload; disconnect/shutdown скасовують роботу. TERM → grace → KILL, wait і cleanup
завершуються до звільнення slot. Runner прибирає також дочірні процеси, що пережили worker;
Compose init reaps orphan processes. HTTP shutdown перериває і незавершені uploads.

Відповідь PDF починається після перевірок. Додано всі контрактні headers, safe JSON errors,
health/readiness (pinned tool versions, worker executable, writable temp), safe structured logs.
`ENABLE_LOSSY=false` працює без gs; тільки явно вибраний lossless дозволений.

Job directories 0700, files 0600 (worker umask 0077), випадкові names. Startup прибирає
тільки `pdfjob-*` із marker у виділеній папці; сторонні entries зберігаються. Watcher кожні
20 ms обмежує сумарний job size 512 MiB; також final check. Короткий overshoot можливий;
Compose tmpfs задає сумарну hard storage boundary. Tools TMPDIR/cache належать job і прибираються.

## Змінені файли та interfaces

Нові файли:

- `internal/config/{config.go,config_test.go}` — defaults/env validation та bounds.
- `internal/storage/{temp.go,temp_test.go}` — `Store.Startup/Writable/NewJob`, `Job.CheckSize/Cleanup`.
- `internal/worker/protocol.go` — private bounded JSON Request/Response, server-owned paths/profile/tools.
- `cmd/pdfworker/{main.go,main_test.go}` — inherited tool group, pipeline/error boundary.
- `internal/httpapi/{server.go,process.go,compress_test.go,process_test.go}` —
  `New`, `Server.Handler/Stop`, `Processor.Process`, `ProcessWorker.Process`; HTTP/admission/IPC/watchdog.
- `internal/observability/logging.go` — JSON logger і generated request IDs; без PDF/tool stderr.
- `cmd/server/main.go` — HTTP timeouts, startup/shutdown.
- `compose.yaml`, `.env.example`, `README.md`, `docs/openapi.yaml` — runtime/API запуск та контракт.
- `tests/e2e/api_test.go` — реальні local-loopback HTTP/process tests у runtime harness.

Змінені існуючі:

- `internal/worker/runner_linux.go` — bounded WaitDelay, synchronized cancellation watcher,
  остаточний group cleanup. Не залишає child, якщо parent завершився раніше grace.
- `internal/qpdf/json.go` — typed `ErrDocumentLimit` для HTTP 422, існуючі caps збережені.
- `internal/compression/service.go`, `service_test.go` — `CompressRequest.MaxPages`;
  required qpdf write/validation failures більше не маскуються success/original;
  tool launch failure відрізняється від invalid source. Мінімальна правка Stage 05 для HTTP error contract.
- `Dockerfile` — server/pdfworker builds, runtime і api-test targets, binary checksums;
  runtime успадковує тільки probe tools/resources/licenses (не QA Poppler/Python).
- `Makefile` — runtime/e2e targets.
- `docs/test-dependency-audit.md`, stage 06 plan/results і тільки tracker row Stage 06.
- `decisions.md` — зафіксовано локальну Docker compatibility option і mandatory-error clarification.

Публічний контракт — [OpenAPI](../../../openapi.yaml), [README](../../../../README.md).
IPC не є HTTP endpoint. Жодних DB/auth/Redis/remote APIs/migrations додано або запущено.

## Перевірки та докази

Усі докази — [artifacts/stage-06](../../../../artifacts/stage-06/), папка gitignored,
тільки дозволені synthetic fixtures. Audit перед КОЖНИМ unit run —
[docs/test-dependency-audit.md](../../../test-dependency-audit.md), A1–A4; e2e review E1–E4.

| Перевірка | Фактичний результат | Доказ |
|---|---|---|
| Unit dependency review | Pass: stdlib/local modules, private temp/fake tools, no DB/init/TestMain/external endpoints | audit + dependency-inventory.txt |
| Unit race A2/A3 | Pass, exit 0, усі 8 internal test packages | unit-race-2.log, unit-race-final-internal.log |
| Final storage/worker boundary A4 | Pass, exit 0, race enabled | unit-race-final-boundary.log |
| Real container e2e E4 | Pass, exit 0, усі 5 top-level tests та підсценарії | e2e-final.log |
| compressed / unchanged / CMYK fallback | Pass: qpdf check кожної відповіді; unchanged exact bytes, correct warning/engine/counts | e2e-final.log |
| Busy / malformed / invalid / timeout / disabled | Pass: 503 Retry-After до body; 400/415/422/504; qpdf-only ready | unit + e2e logs |
| Cancel/process groups обох tools | Pass: qpdf worker/tool 2254/2261, PGID 2254; gs 2276/2290, PGID 2276; всі observed PIDs reaped | e2e-final.log |
| TERM-resistant descendants/storage limit | Pass: fake tools ignore TERM, watcher/deadline kill group, exact child PID disappears | httpapi ProcessWorker tests, unit-race-final-internal.log |
| Shutdown / partial upload cleanup | Pass: unit Stop і real server shutdown interrupt read; jobs empty | e2e-final.log |
| Temp permissions/startup isolation | Pass: real 700 job / 600 input+marker; marked leftovers only; unrelated preserved | job-permissions.txt + storage tests |
| Runtime Compose + local HTTP | Pass: ready, 200 application/pdf, full headers; readonly/nonroot/init/resource limits | compose-local-resolved.yaml, container-inspect.json, compact-headers.txt |
| HTTP output structural check | Pass, qpdf exit 0 | output-qpdf-check.txt, compact.pdf |
| Post-response cleanup / logs | Pass: no jobs, only init/server remain, safe bytes/time/engine logs | runtime-evidence.txt, processes-after-request.txt, server.log |
| Test/container cleanup | Pass: own Compose container/network removed; no pdf-stage06 containers remain | compose-down.txt, container-cleanup.txt |

Final manual Compose response: **561006 → 16209 bytes, 97.11%, ghostscript**, request ID
`a398210d06ef4a80cc422d4e35c24c84`. Це конкретний synthetic fixture результат, не quality/load benchmark.
Balanced/lossless і unchanged/fallback окремо підтверджені e2e. Headers не заявляють optimized/skipped counts.

### Exact commands

В цьому середовищі CLI використано напряму: `/snap/docker/current/bin/docker`.
Нижче `docker` означає цей executable; host Go wrapper не використовувався для test acceptance.

```sh
# Перед кожним unit command: inventory + manual closure review + audit entry
./scripts/check-test-dependencies.sh

docker run --rm --init --network none --read-only --cap-drop ALL \
  --tmpfs /tmp:rw,exec,size=1g -e GOCACHE=/tmp/go-cache -e GOPATH=/tmp/go-path \
  -e GOFLAGS=-buildvcs=false -v /home/alex/Desktop/pdf-golang-zip:/src:ro \
  -w /src golang:1.27.2-bookworm go test -race -count=1 ./internal/...
# A4: те саме середовище, command go test -race -count=1 ./internal/storage ./cmd/pdfworker

docker build --target runtime -t pdf-compression:stage06 .
docker build --target api-test -t pdf-compression:stage06-api-test .
docker run --rm --init --network none --read-only --cap-drop ALL \
  --memory 2g --cpus 2 --pids-limit 128 --tmpfs /tmp:rw,exec,nosuid,size=1g \
  pdf-compression:stage06-api-test

PORT=18086 NO_NEW_PRIVILEGES=false docker compose -p pdf-stage06-check up -d
curl --fail --silent --show-error http://127.0.0.1:18086/readyz
curl --fail --silent --show-error -D artifacts/stage-06/compact-headers.txt \
  -o artifacts/stage-06/compact.pdf -F profile=compact \
  -F file=@testdata/pdf/flate-rgb-600dpi.pdf http://127.0.0.1:18086/v1/compress
PORT=18086 NO_NEW_PRIVILEGES=false docker compose -p pdf-stage06-check down
```

### Exact runtime artifacts

- Runtime image: `pdf-compression:stage06`,
  `sha256:bcc9d6b9b8be30d147af2698cc56ab957d2db430f36656e1954784996da4c49f`.
- Final test image: `pdf-compression:stage06-api-test`,
  `sha256:6a404a7f979e864a73a96be34154f98770220d14a011a59400464fdf4e08d219`.
- gs/qpdf/server/pdfworker SHA-256 і actual versions — runtime-evidence.txt;
  pinned build source hashes/digests незмінні в Dockerfile.
- Runtime includes existing GS fonts/resources, source archives, notices/licenses and package manifests.

## Відхилення та обмеження

1. На цьому snap Docker host `--security-opt no-new-privileges` з init повертає
   `exec /sbin/docker-init: operation not permitted` (exit 255). Це не тест Pass.
   Локально використано явне `NO_NEW_PRIVILEGES=false`; Compose default true залишено,
   сумісний host має використовувати default. Повторна перевірка цієї конкретної опції
   на сумісному host потрібна; решта runtime protections реально перевірена тут.
2. Перший unit run мав noexec tmpfs і не міг запустити test binaries (exit 1).
   Тестова папка стала exec; production temp/jobs залишаються noexec.
3. Перший e2e process observation читав children лише leader thread; це пропускало
   Go subprocesses. Test виправлено на all threads, потім qpdf/gs окремо пройшли.
   Failed attempts збережені, не зараховані як proof.
4. Watcher — polling, не per-job filesystem quota; контейнерний tmpfs/resource bound загальний.
   GOMEMLIMIT worker — soft limit. Практична поведінка двох важких jobs — Stage 07.
5. Qpdf validity і runtime text/semantic checks не доводять повну visual fidelity.
   Широкі fixtures, quality/load acceptance не виконувалися у цьому stage.

## Передача

Stage 06 локальні функціональні критерії виконано. Baseline для Stage 07 — файли вище,
фінальний runtime image, audit і artifacts/stage-06. Загальна release readiness ще не заявлена.
Наступний етап: **Stage 07 — quality/load**, `07-quality-and-load.md`.
Stage 07/08 не реалізовувалися; нових чатів, commits, deployment або міграцій не створено.
