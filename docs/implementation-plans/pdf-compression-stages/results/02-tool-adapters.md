# Результат Stage 02: Adapters Ghostscript/qpdf та процеси

- Статус: **Completed**.
- Останнє оновлення: 2026-10-09.
- Чат/виконавець: поточний chat розробки.
- Версія плану: 2026-10-09; редакція GS-v1; engine baseline — Go 1.27.2 + open-source Ghostscript 10.08.0 + qpdf 12.4.2.
- Baseline: Stage 01 probe/QA завершено; Git repository недоступний (`git status`: not a git repository); commit/push не виконувалися.

## Фактичний результат

Реалізовано безпечні та обмежені (bounded) адаптери для CLI-утиліт `qpdf` та `gs` (Ghostscript), а також runner процесів під Linux:
1. `internal/worker`: реалізовано Linux runner із керуванням групами процесів (`Setpgid: true`), підтримкою graceful termination (SIGTERM -> 500ms grace -> SIGKILL) для всього дерева дочірніх процесів, обмеженими буферами (`BoundedBuffer` з дефолтним лімітом 64 KiB для діагностики stderr/stdout) та структурованими помилками (`ExitError`).
2. `internal/qpdf`: реалізовано `Client` з методами `IsEncrypted` (розрізнення кодів: 0 = encrypted, 2 = not encrypted), `Check` (exit 0 проти 2/3), `Analyze` та `AnalyzeRaw` (запуск `qpdf --json=2 --json-stream-data=none` з лімітом 64 MiB), `ReadStream` (запуск `qpdf --show-object=<id> --filtered-stream-data` з лімітом 4 MiB для XMP), `WriteLossless` (перепакування зі стандартними прапорцями `--compress-streams=y --decode-level=generalized --recompress-flate --compression-level=9 --object-streams=generate`). Парсер JSON v2 використовує `json.Decoder.UseNumber()` для точного збереження чисел та безпечно обробляє службові секції метаданих.
3. `internal/ghostscript`: реалізовано `Client` з методами `Rewrite` (запуск `gs` pdfwrite з `-dSAFER`, довіреними `--permit-file-read` та `--permit-file-write`, baseline balanced параметрами) та `ExtractText` (запуск `gs` txtwrite `-dTextFormat=3`, обмеження виходу 32 MiB, нормалізація `NormalizeText`: CRLF/CR -> LF, видалення trailing spaces/tabs, обрізання кінцевих порожніх рядків).
4. `cmd/engine-probe/main.go` оновлено: тепер probe використовує створені адаптери `worker.Runner`, `qpdf.Client` та `ghostscript.Client`.
5. Dockerfile оновлено: додано копіювання `internal/` у етап збірки `go-build`. Образ `pdf-engine-stage01:probe` успішно перезібрано та перевірено в контейнері — 16/16 перевірок пройдено (Pass).

## Змінені файли та прийняті рішення

- `internal/worker/runner.go` — інтерфейс `Runner`, `CommandOptions`, `Result`, `ExitError`, `BoundedBuffer`.
- `internal/worker/runner_linux.go` — Linux-реалізація із `Setpgid`, `SIGTERM`/`SIGKILL` та reap групи процесів.
- `internal/worker/runner_other.go` — переносимий fallback для не-Linux середовищ.
- `internal/worker/runner_test.go`, `internal/worker/runner_linux_test.go` — юніт-тести runner, буферів та дерева процесів.
- `internal/qpdf/client.go` — клієнт qpdf: `IsEncrypted`, `Check`, `Analyze`, `AnalyzeRaw`, `ReadStream`, `WriteLossless`.
- `internal/qpdf/json.go` — структури qpdf JSON v2 (`JSONOutput`, `PageInfo`, `ImageInfo`, `OutlineInfo`), збереження `UseNumber()`, `FindObject`, `Root`, `Trailer`.
- `internal/qpdf/objects.go` — `ObjectEntry`, `NormalizeObjectID`.
- `internal/qpdf/streams.go` — константи лімітів (JSON 64 MiB, XMP 4 MiB, stream 32 MiB), `ReadBounded`.
- `internal/qpdf/client_test.go`, `internal/qpdf/json_test.go`, `internal/qpdf/streams_test.go` — ізольовані тести з fake runner.
- `internal/ghostscript/client.go` — клієнт Ghostscript: `Rewrite`, `ExtractText`, `NormalizeText`, перевірка діагностики.
- `internal/ghostscript/client_test.go` — юніт-тести Ghostscript з fake runner.
- `cmd/engine-probe/main.go` — інтеграція нових адаптерів у probe.
- `Dockerfile` — копіювання `internal/` у збірку Go.
- `Makefile` — оновлення цілі `unit` для виконання аудиту та `go test -v ./...`.
- `docs/test-dependency-audit.md` — зафіксовано запис аудиту залежностей перед виконанням unit-тестів.
- `docs/implementation-plans/pdf-compression-stages/README.md` — оновлено статус Stage 02 у tracker.

## Перевірки

| Перевірка | Команда або ручна процедура | Фактичний результат | Артефакт / доказ |
|---|---|---|---|
| Unit dependencies audit | `./scripts/check-test-dependencies.sh` + manual review | Pass: 0 DB/network/external imports | `docs/test-dependency-audit.md` |
| Unit tests (worker, qpdf, ghostscript) | `go test -v ./...` | Pass: 100% тестів успішно (exit 0) | вивід `make unit` |
| Child tree process termination | `TestLinuxRunnerProcessGroup` | Pass: дочірнє дерево sleep завершено за cancel | вивід `go test -v ./internal/worker` |
| qpdf encryption exit codes | `TestClientIsEncrypted` | Pass: exit 0 -> true, exit 2 -> false | `internal/qpdf/client_test.go` |
| Bounded streams & txtwrite cap | `TestClientReadStream`, `TestGhostscriptExtractText` | Pass: caps 4 MiB та 32 MiB повертають помилку | `internal/qpdf/client_test.go`, `internal/ghostscript/client_test.go` |
| Text normalization | `TestNormalizeText` | Pass: CRLF->LF, trim spaces, trim blank lines | `internal/ghostscript/client_test.go` |
| Probe compilation in Docker | `docker build --target probe -t pdf-engine-stage01:probe .` | Pass: exit 0 | Docker image `pdf-engine-stage01:probe` |
| Real tool probe run with adapters | `make probe RUN_ID=stage02-test SECURITY_OPT=` | Pass: 16 observations, status Pass | `artifacts/stage-01/stage02-test-probe/report.json` |

## Критерії завершення

1. **Обидва adapters запускаються без shell:** Доведено. Усі виклики використовують прямі масиви аргументів `exec.Command` через `worker.Runner`.
2. **Malformed/errors/caps коректно mapped:** Доведено. `ExitError` зберігає коди та діагностику; `ErrCheckFailed`, `ErrStreamExceedsLimit`, `ErrTextLimitExceeded`, `ErrInvalidFileAccess` явно розрізняються.
3. **txtwrite 32 MiB cap:** Доведено. `ExtractText` перевіряє ліміт та повертає `ErrTextLimitExceeded` при перевищенні.
4. **Дочірнє дерево завершується при cancel:** Доведено юніт-тестом `TestLinuxRunnerProcessGroup` із надсиланням `SIGTERM` та `SIGKILL` у process group.
5. **Probe використовує adapters:** Доведено. `cmd/engine-probe/main.go` переведено на `qpdf.Client`, `ghostscript.Client` та `worker.Runner`, успішно протестовано в Docker.

## Відхилення, дефекти та обмеження

1. У qpdf JSON v2 перша секція масиву `"qpdf"` містить службові метадані (`{"jsonversion": 2, ...}`). Тип поля `QPDF` у `JSONOutput` визначено як `[]map[string]json.RawMessage`, що дозволяє коректно парсити як метадані, так і indirect objects без помилок десеріалізації.
2. Профілі Ghostscript у цьому stage використовують фіксовані baseline balanced settings; повноцінний генератор конфігурацій профілів належить Stage 04.

## Передача наступному чату

- **Наступний етап:** Stage 03 — Аналіз PDF та політика MVP (`03-analysis-and-policy.md`).
- **Готові інтерфейси:**
  - `internal/worker.Runner`, `internal/worker.LinuxRunner`, `internal/worker.CommandOptions`
  - `internal/qpdf.Client`: `Analyze`, `AnalyzeRaw`, `IsEncrypted`, `Check`, `ReadStream`, `WriteLossless`
  - `internal/qpdf.JSONOutput`, `internal/qpdf.ObjectEntry`, `internal/qpdf.PageInfo`
  - `internal/ghostscript.Client`: `Rewrite`, `ExtractText`, `NormalizeText`
- **Вимоги для Stage 03:** Реалізувати `internal/pdf/{analyzer,policy,inventory,snapshot}.go` без CTM lexer та без сторонніх залежностей.
