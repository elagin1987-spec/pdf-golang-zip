# Test dependency audit — Stage 01, 2026-10-09

Reviewed before the first real integration probe:
`cmd/engine-probe/main.go`, `go.mod`, `Dockerfile`, `Makefile`,
`testdata/pdf/generate.py`, `testdata/pdf/manifest.json`, and
`scripts/check-test-dependencies.sh`.

* No `_test.go`, TestMain, init function, helpers, service configuration,
  external Go modules or `go.sum` exist. The Go program uses only standard
  library packages. DB drivers/clients, Redis, migration tools, environment
  credentials and network clients are absent.
* Allowed writes: a newly created artifact directory, synthetic input copies,
  candidate PDFs, text, JSON, renders and the report; fixture regeneration
  replaces only owned PDFs and their manifest under `testdata/pdf/`.
* Commands are fixed local gs/qpdf/Poppler binaries with argument arrays.
  No shell or user PDF input is accepted. The probe is an integration check,
  with networking disabled, read-only root filesystem and resource limits.
* Build-time network downloads are only official source/dependency packages.
  Document processing performs no network calls. No DB connection or data
  changes, production documents or migrations are involved.
* Inventory script was run successfully; source and all imports were reviewed
  manually. The script is an aid and does not certify dependency safety.

Unit runs: **Not run; Stage 01 introduces no unit tests.** Before adding or
running unit tests in a later stage, repeat the full dependency/import/helper/
init/TestMain/config audit and append a dated record. No automatic unit target
bypasses that requirement.

2026-10-09, review update before QA runs: re-read the updated probe after adding
fixed `pdffonts`/`pdfimages` invocations, read/write SAFER denial and explicit
semantic-fallback reporting. Imports and dependency closure remain standard
library only. Added commands are local reads of owned PDFs. Writes remain inside
the new artifact directory. No unit test runs, DB clients or migrations added.

2026-10-09, review before final pinned QA: added standard-library image/png only
for comparing independently extracted QA PNG pixels. It performs no resize,
network or database operation. Reviewed the decoder and file-open paths; inputs
are owned 1200-square fixtures, writes remain local artifacts/tmpfs. Repeated
integration probes are justified by the Fontconfig cache fix, corrected JBIG2
fixture framing/polarity and final snapshot image. No unit tests were run.

## Stage 02 — 2026-10-09: Unit test dependency audit

Reviewed before running Stage 02 unit tests:
`internal/worker/{runner,runner_linux,runner_other,runner_test,runner_linux_test}.go`,
`internal/qpdf/{client,json,objects,streams,client_test,json_test,streams_test}.go`,
`internal/ghostscript/{client,client_test}.go`, and updated `cmd/engine-probe/main.go`.

* `scripts/check-test-dependencies.sh` inventory was executed and verified.
* Zero external Go modules: `go.mod` remains standard library only; `go.sum` absent.
* Zero database interaction: no `database/sql`, GORM, pgx, Redis, Mongo, or migrations.
* Zero network interaction: no network sockets, HTTP clients, or remote endpoints.
* Zero `TestMain` or `func init()` hooks in any package.
* All tests are fully isolated unit tests using fake runners or standard subprocess calls
  (`echo`, `sleep`, `sh -c`) for process group/signal lifecycle verification.
* File writes in tests are strictly confined to isolated `t.TempDir()`.
* Conclusion: Dependency closure is clean and safe for unit test execution.

## Stage 03 — 2026-10-09: Unit test dependency audit

Reviewed before running Stage 03 unit tests:
`internal/pdf/{policy,inventory,snapshot,analyzer}.go` and
`internal/pdf/{policy_test,inventory_test,snapshot_test,analyzer_test}.go`.

* `scripts/check-test-dependencies.sh` inventory was executed and verified.
* Zero external Go modules: `go.mod` standard library only; `go.sum` absent.
* Zero database interactions: no SQL, GORM, database drivers, Redis, or migrations.
* Zero network sockets, HTTP clients, or remote connections.
* Zero `TestMain` or `func init()` hooks in `internal/pdf`.
* All new tests in `internal/pdf` are 100% in-memory unit tests using mock runners and synthetic JSON structures.
* Zero filesystem writes performed by `internal/pdf` tests.
* Conclusion: Dependency closure is verified clean and safe for unit test execution.

## Stage 04 — 2026-10-09: Unit test dependency audit

Reviewed before running Stage 04 unit tests:
`internal/ghostscript/{profiles,settings,diagnostics,client}.go` and
`internal/ghostscript/{profiles_test,settings_test,diagnostics_test,client_test}.go`.

* `scripts/check-test-dependencies.sh` inventory was executed and verified.
* Zero external Go modules: `go.mod` standard library only; `go.sum` absent.
* Zero database interactions: no SQL, GORM, database drivers, Redis, MongoDB, or migrations.
* Zero network sockets, HTTP clients, or remote network connections.
* Zero `TestMain` or `func init()` hooks across all packages.
* All new tests in `internal/ghostscript` are 100% in-memory unit tests using string comparisons, fake runners, or isolated `t.TempDir()`.
* Zero external data writes or background threads.
* Conclusion: Dependency closure is verified clean and safe for unit test execution.

## Stage 05 — 2026-10-09: Unit test dependency audit

Reviewed before running Stage 05 tests:
`internal/compression/{profiles,result,validation,service}.go`,
`internal/compression/{profiles_test,validation_test,service_test,test_helpers_test}.go`, and
`tests/integration/compress_test.go`.

* `scripts/check-test-dependencies.sh` inventory was executed and verified.
* Zero external Go modules: `go.mod` standard library only; `go.sum` absent.
* Zero database interactions: no SQL, GORM, database drivers, Redis, MongoDB, or migrations.
* Zero network sockets, HTTP clients, or remote network connections.
* Zero `TestMain` or `func init()` hooks across all packages.
* All tests in `internal/compression` are 100% isolated unit tests using fake runners and isolated `t.TempDir()`.
* `tests/integration/compress_test.go` uses local pinned tools and local fixture files without external network calls or database connections.
* Zero external data writes or unmocked external dependencies.
* Conclusion: Dependency closure is verified clean and safe for test execution.

## Stage 06 — audit A1, 2026-10-09, before first unit run

Reviewed module, test imports, helpers, fake runners, initialization/config and dependency closure
for `internal/{config,storage,httpapi,worker,compression,pdf,qpdf,ghostscript}`.
Inventory: `scripts/check-test-dependencies.sh`; interaction search includes TestMain/init,
SQL/drivers/Redis/Mongo, HTTP/dial, subprocess and filesystem writes.
No external modules, DB clients, migrations, TestMain or init hooks. HTTP tests use
httptest Recorder/Request in memory; processor injected except private tool stubs.
Worker tests execute echo/sh/sleep/head and version stubs only; files under t.TempDir.
Configuration reads only process environment, restored by t.Setenv. No DB/network access.
Execution isolated in Go image with network none, init, read-only source mount, temporary
build/cache paths; integration/e2e are excluded from this unit command.
Command A1: `go test -race -count=1 ./internal/...` in pinned Go container. Safe to run.

A1 result: no test binary could execute (tmpfs defaults to noexec); exit 1, NOT a test pass.
A2 review before retry: same test/import/helper/config closure, no source changes since A1;
no DB/network dependencies. Only temporary test filesystem changes to exec. Command:
`go test -race -count=1 ./internal/...` in network-none read-only Go container, tmpfs exec.

A2 result: all eight test packages passed with race detector (exit 0).
A3 review before next run: unchanged tests plus same reviewed dependency closure;
changes since A2 are storage watcher ENOENT tolerance and HTTP IO deadlines on cancellation,
and safe mapping of watcher storage errors. No DB/driver/client/init hooks.
Command A3: `go test -race -count=1 ./internal/...` in the same isolated Go container.

E1 review: `tests/e2e/api_test.go` uses only standard library, local server/worker processes,
synthetic generated PDFs and checked-in fixtures, qpdf --check in temp folders, HTTP loopback,
local /proc reads and signals to the server PID created by this test. No remote endpoints,
DB, migrations, private documents or writes outside container-private temp. Execute
in api-test runtime image with network none (loopback remains enabled), init and read-only root.
E2E tests are real runtime checks, distinct from fake-tool unit evidence.

A3 result: all eight test packages passed with race detector (exit 0).
E1 follow-up review before execution: e2e now also checks local /proc child PGID and
absence of both worker/tool PID after disconnect; no additional imports or external writes.

E1 attempt: container did not start (`exec /sbin/docker-init: operation not permitted`, exit 255)
with no-new-privileges on this snap Docker host. E1 retry review: same e2e sources/helpers,
no dependency changes; omit only that host-incompatible security option locally.

E1 retry result: HTTP profiles/unchanged/CMYK fallback/busy/timeout/lossy-disabled/shutdown
passed; processing-disconnect observation failed because /proc leader-thread children omitted
children created by other Go threads. This was a test observation defect, not proven cancellation.
E2 review before rerun: processChildren now reads `/proc/PID/task/*/children` for only this
server/worker; unchanged local-only dependencies, no DB/network outside loopback. Repeat e2e.

E2 result: all five top-level e2e tests passed (exit 0), including real tool shared process
group and disconnect reaping; Linux observation corrected to all threads.
A4 review before targeted final tests: `cmd/pdfworker/main_test.go` imports context/fmt/os/exec/testing
and compression/qpdf only; error injection in memory, no executable tool calls. Reviewed main/error
mapping and previously audited closure. `storage/temp_test.go` adds a private deleted-job case;
filesystem writes/removal remain under t.TempDir. No DB/init/TestMain/network dependencies.
Command A4: `go test -race -count=1 ./internal/storage ./cmd/pdfworker` in isolated Go container.
E3 review before final e2e: product changes distinguish missing tool from lost input, and detect
lost job root (temporary child ENOENT still tolerated). All e2e helpers/imports unchanged;
local loopback, synthetic fixtures, local CLI and private temp only, no DB. Repeat real e2e.

A4 result: storage and worker boundary tests passed with race detector (exit 0).
E3 result: full real e2e suite passed after final product changes (exit 0).
E4 review before final evidence run: process-disconnect test now repeats the same local /proc,
loopback/PDF workflow specifically for qpdf and gs, so both CLI groups are independently observed
and canceled. No dependency, product code, endpoint or data source changes. Repeat e2e.

E4 result: all real e2e tests passed (exit 0); both qpdf and gs observed in their worker group,
then canceled/reaped, jobs removed. Product sources unchanged since A4. No migrations/DB calls.
Final evidence: artifacts/stage-06/{unit-race-final-internal.log,unit-race-final-boundary.log,e2e-final.log}.

## Stage 07 — 2026-10-09

S07-U1, before unit execution: current go.mod has only the local module and Go 1.27.2;
no go.sum/external modules. Reviewed inventory imports and interaction scan, fakeRunner
helpers in compression/qpdf/ghostscript/pdf, config env reads, storage t.TempDir cleanup,
HTTP httptest processor/readiness stubs, local fake shell tools in process tests and
worker process-group tests. No database/sql, drivers, ORM, Redis, init or TestMain hooks.
Production closure is stdlib + local packages; no DB/external service configuration.
Unit filesystem writes stay under private test temp; process tests spawn only echo/sh/sleep
and inspect local PIDs. Run `go test -race -count=1 ./internal/... ./cmd/pdfworker` in
network-none, read-only Go container, private exec tmpfs. Inventory evidence:
artifacts/stage-07/dependency-{inventory,review}.txt. No migrations allowed or run.

S07-R1, before integration/e2e/QA: new matrix tests import stdlib and the same local
compression/pdf/tool packages. Manifest/PDF reads use only owned fixtures with SHA-256;
qpdf/gs processes write private temp, API HTTP uses only local loopback and owned server
PIDs. Heavy cancel test reads generated synthetic load fixture, signals only its requests,
and observes its own worker/tool children. QA Python uses stdlib/Pillow, local tools,
/read-only source and dedicated artifact output; local server only. zbar is QA-only.
No DB or remote processing, no user documents. Fake-tool unit tests are not codec evidence.
S07-U1 result: all eight internal packages and cmd/pdfworker passed with race detector,
exit 0; artifacts/stage-07/unit-race.log. No product sources changed during Stage 07.
S07-R2 review before real-tool rerun: only fixture form/annotation dictionaries changed
to valid indirect objects and QA render screening records a documented Indexed-pattern
MAE 8/255 (manual reviewed downsample/chroma-edge differences). Added policy negatives,
all owned raw synthetic PDFs. Go/production dependencies unchanged; no DB or remote calls.
R1 failed fixture preflight is not counted as a pass. Repeat matrix and independent QA.
S07-R2 results: integration matrix passed; real API matrix passed. Heavy cancel failed
because generated max-load PDF had padding after EOF beyond qpdf's startxref search;
not evidence of production failure. Independent QA: all 96 rows passed, but cross-codec
mono comparison failed due to CCITT fixture BlackIs1 polarity. Overall runs Fail.
S07-R3 review: generator moves 50MiB padding before xref and corrects CCITT BlackIs1;
QA also compares raw GS mono pixels, and load instrumentation adds anon/shmem/allocated
bytes. All imports/helpers/init/config/production unchanged; synthetic local artifacts only.
Repeat final matrix, symbol/mono QA and max-load cancel; then run load without concurrent
QA/tests to avoid performance interference. No further unit rerun required: production and
unit test sources unchanged since U1. Before real runs, review completed as above.
S07-R3 result: integration and all real e2e tests passed (exit 0), including 96 HTTP
matrix subtests and busy/cancel/cleanup with 1/2 workers at exactly 500 pages / 50 MiB.
Independent Poppler/zbar QA passed all 96 rows; raw GS mono equality 8/8 and CCITT/JBIG2
cross-decoder hashes equal. Evidence: real-tests-accepted.log, quality-accepted/{quality.json,
mono-codec-crosscheck.json,quality-commands.json}. No production/unit changes since U1.
S07-L1 review before load: same reviewed local Python stdlib/Pillow and actual server,
worker, gs/qpdf tool closure; no new endpoints or imports. Only local loopback POSTs,
owned load PDFs and dedicated tmpfs jobs. 20ms cgroup/proc/disk observations are read-only;
test output is artifact-only. Network-none, root readonly, caps/cpu/pids/memory unchanged.
S07-L1 result: 12 positive batch cases / 4 refusal cases, all Pass; dedup 12 real GS
invocations; load overall exit 0. Artifacts benchmark.json/dedup.json/load-commands.json.
Independent saved-feature checker uses pypdf/stdlib read-only source/result PDFs and writes
only independent-features.json: 69 comparisons, 0 failures. No network/database hooks.
Final QA-only hardening: preserve existing acceptance output dirs, daemonize monitor thread
on exceptional exit, record Poppler version from already-captured stderr, unique Make run
folder. No numerical algorithms/product/unit dependency changes; accepted measurements
are unchanged. No additional unit run.

## Stage 08 — 2026-10-09 / S08-D1

No unit test run. Stage 07 baseline verification matched all 125 source files before
editorial/config changes; no Go source/test/helper/init/TestMain/dependency changes.
Retained Stage 07 unit/race evidence, no repeated unit acceptance claim.
New smoke uses existing accepted server/worker/qpdf binaries, owned synthetic
flate-rgb-600dpi and special-tagged PDFs, local loopback and private Compose project.
No database client/dependency/migrations/data writes or external APIs; only temp jobs,
local Docker image save/load and handoff artifacts. Every own smoke container/network
removed. Runtime inventory container uses network none. Config/docs/package verification
is not a unit run. Prior dependency closure and audit remain in the Stage 06/07 entries.

## Policy/validation fixes — 2026-10-09 / F1 before regression run

Reviewed new policy_regression_test.go, xmp_regression_test.go and
error_regression_test.go, existing pdf/compression test helpers, qpdf/ghostscript
clients, worker Runner, go.mod and inventory. Only stdlib/local modules, no DB,
ORM/drivers, migrations, init/TestMain or remote services. New tests use synthetic
JSON, fake runners and writes under t.TempDir; no real tools are launched by these
selected tests. Run only regression names in pdf/compression with pinned Go,
network none, read-only source, private executable temp/cache; expected failures
establish reproduction and must not be counted as accepted results.

F1 could not execute Go (operation not permitted), exit 255: not a test result.
F1b review: identical audited sources/imports/helpers; retry omits only locally
incompatible no-new-privileges option. Network none/read-only/caps/tmp retained.

F1b result: regressions fail on all three original behaviors, exit 1 (expected).
F2 before targeted run: reviewed changed policy bounded dictionary/array traversal,
XMP error handling, qpdf error sentinel/classification, service error propagation,
new qpdf classification tests and existing helpers/dependency closure. No new modules,
DB/init/TestMain/network or external data writes. Fake runners only in regression
cases; existing target suites use synthetic JSON/private temp. Run pdf/qpdf/compression/
pdfworker packages in same isolated pinned Go container, without local-incompatible
no-new-privileges; all other isolation retained. Updated inventory saved separately.

F2 result: targeted pdf/qpdf/compression/pdfworker suites pass, exit 0.
F3 before full unit/race run: re-reviewed final Go changes plus wrapped worker
boundary cases; inventory includes all internal packages and pdfworker. Existing
config tests read environment via t.Setenv; storage writes private t.TempDir;
HTTP uses httptest/injected processors and private shell/version stubs; worker
lifecycle tests launch only echo/sh/sleep/pgrep inside isolated container and signal
own PIDs/groups. No DB drivers/modules, init/TestMain or remote endpoints.
New regressions remain fake-runner-only and use owned temp files. Full race run:
go test -race -count=1 ./internal/... ./cmd/pdfworker; pinned Go/network none,
read-only source, private exec tmpfs/cache, unchanged caps. No migrations.

F4 before real integration/e2e/quality: reviewed new stdlib fixture generator and
HTTP regression suite: eight owned PDFs/checksums, qpdf check, local loopback,
owned child server, private temp results and cleanup. Existing real matrix/max-load
helpers unchanged: no remote endpoint/DB, signals target their own children.
Independent quality script unchanged: Poppler/zbar/Pillow/local CLI only. Reuse
owned Stage 07 load fixtures read-only for max-load busy/cancel. New image builds
current sources; old runtime is not used as evidence of fixes.

F3 result: changed packages passed race; HTTP descendant-reaping tests failed
because test container lacked init. This is not an accepted full run. Production
Compose already uses init; no production fix is inferred from this harness error.
F3b before retry: same Go sources/tests/dependency closure as F3, no DB/init hooks
or remote services introduced. Add Docker --init to reap orphaned descendants,
retaining network none/read-only/cap-drop/resource/private tmpfs settings. Repeat
full internal/pdfworker race suite; local no-new-privileges exception unchanged.

F3b result: all internal/pdfworker packages pass race with init, exit 0.
F4 real run failed: new policy walker treated qpdf scalar JSON header fields as
ObjectEntry values. Corrected to scan obj:* and trailer only; added real-format
header control to policy regression test. No prior failed real run is acceptance.
F5 before final unit/race: reviewed header filtering and regression control;
otherwise same full F3b dependency/helper/config/process closure. No DB/network/
init/TestMain or new modules. Repeat full race in identical init/network-none,
read-only/resource-limited pinned Go container. Updated inventory saved.
F6 before real rerun/quality: same F4 real/Python dependencies, new header filter
only. Current Docker images will be rebuilt; readonly current source, owned
fixtures/local loopback/private tmpfs remain. No external documents or DB.

F5 result: all internal/pdfworker packages pass full race, exit 0.
F6 real result: integration and e2e pass, exit 0, no skips; 96 existing HTTP
matrix cases and 24 new nested-action/control cases, plus max-load busy/cancel.
Final QA image reuses accepted Stage 07 pinned QA tools and copies both current
runtime binaries; each run mounts current workspace readonly at /src. Runtime/
QA image IDs and binary/source hashes are recorded in the new evidence folder.
F7 before independent semantic/visual comparison: bundled pypdf + stdlib checker
reads saved quality source/outputs and writes new independent-features.json;
PNG equivalence checker reads only owned previous/new render files. No DB/network.
F8 before new load benchmark: re-reviewed unchanged quality_load.py benchmark,
fixture generation, loopback HTTP, cgroup/proc/disk read-only observations and
server stop/cleanup. Output confined to new evidence/load-accepted; dedicated
runtime jobs tmpfs, network none/read-only/caps/2GiB/2CPU/128PID retained. Full
load rerun justified by added bounded policy traversal cost, with no concurrent QA.
No migrations, remote documents or external processing services.

F7 result: 96 quality rows Pass; 69 independent pypdf comparisons exact/tolerance;
109 current page PNGs byte-identical to prior accepted/manual-reviewed corpus.
F8 first load attempt failed before first upload: expected load-fixtures directory
was not supplied. No benchmark acceptance. Own server stopped in finally.
F8b before retry: unchanged audited benchmark/server/tool closure; bind the owned
Stage 07 load-fixtures readonly at fresh load-accepted-retry/load-fixtures. No fixture
regeneration, writes only new output and own tmpfs jobs; no DB/remote calls. Preserve
failed attempt. Run alone after quality/semantic checks complete.

F8b result: fresh load 16/16 Pass, dedup 12 calls, exit 0. Peak sampled memory
935.4MiB, aggregate job disk 502.4MiB, single job 272.7MiB within existing caps.
QA container list empty after checks; no service left running. Production/source
hash snapshot still matches accepted build; final changes are report/docs only.
