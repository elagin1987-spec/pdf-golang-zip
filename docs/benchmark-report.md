# Stage 07 — benchmark report

Нові вимірювання після policy/XMP/qpdf fixes:
[16/16 Pass, peak memory 935.4 MiB](policy-validation-fixes-report.md).
Наведений нижче Stage 07 benchmark збережено як історичний.

Дата: 2026-10-09. **Pass** для локального synthetic bounded workload.
Реальні HTTP server/pdfworker/Ghostscript/qpdf; Docker network none, read-only root,
non-root UID 1000, init, cap-drop ALL, **2 CPU / 2 GiB / 128 PIDs**.
Production job tmpfs **1200 MiB, noexec**, /tmp **256 MiB, noexec**; кожен job
**512 MiB**, processing timeout **120 s**, file **50 MiB**, pages **500**, workers **1/2**.
Жоден cap не підвищено. Default no-new-privileges на цьому snap-host не працює;
збережений documented Stage 06 compatibility exception, не Pass цієї security option.

## Corpus і метод

[Load manifest](../artifacts/stage-07/load-fixtures/manifest.json): seeded owned patterns,
500-page text PDF; **500 pages / exactly 52,428,800 bytes** with 500 independent RGB images;
501-page refusal; 64 independent image objects with duplicate/unique pixels on 8 pages.
50MiB PDF має valid xref, padding before xref; не простий malformed size placeholder.

Load run виконувався після завершення QA/e2e, без паралельних project tests.
Для MAX_WORKERS=1 — один запит; =2 — два concurrent запити з перекриттям active jobs,
**max_active_jobs** дійсно 1/2 у всіх cases. Всі profiles для max-size PDF;
решта PDF — balanced. Один trial на API case: немає статистичного p95/throughput promise.

20ms sampler: cgroup memory.current, memory.stat anon/shmem, сумарні job logical/allocated
bytes, max single-job logical bytes. Peaks — **sampled lower bounds**, не exact kernel
high-water mark. Cgroup memory включає tmpfs/page cache і test harness; anon окремо.
Logical disk — сума file lengths (allocated blocks також записано). Persistent artifacts
не входять у job-disk count; можуть впливати на page-cache memory.

HTTP total includes streaming upload/download; колонка processing — від кінця надсилання
upload до response headers, наближена до server processing, не GS-only CPU time.
Safe server request-ID/duration logs — `server-1.log`, `server-2.log`; detailed measurements:
[benchmark.json](../artifacts/stage-07/benchmark.json), [load.log](../artifacts/stage-07/load.log).

## Фактичні вимірювання

Всі нижченаведені HTTP replies **200**, returned bytes <= input, qpdf check exit 0,
cleanup завершено. Для 500-page PDF Poppler text/form-feed/order exact також перевірено.
Час wall batch включає завершення запитів, sampler і cleanup; processing — найдовший request.

| Workers/jobs | Fixture / profile | Batch s | Processing s | Cgroup MiB | Anon MiB | Job disk MiB | Savings % | Result |
|---|---|---:|---:|---:|---:|---:|---:|---|
| 1 | pages-500.pdf / balanced | 1.461 | 1.457 | 49.8 | 46.8 | 0.5 | 54.79 | Pass |
| 1 | pages-500-50mib.pdf / lossless | 1.340 | 1.221 | 144.7 | 27.0 | 99.7 | 0.66 | Pass |
| 1 | pages-500-50mib.pdf / balanced | 8.773 | 8.668 | 379.1 | 51.7 | 273.3 | 0.66 | Pass |
| 1 | pages-500-50mib.pdf / compact | 7.918 | 7.807 | 410.7 | 56.0 | 249.2 | 0.66 | Pass |
| 1 | dedup-unique.pdf / balanced | 1.935 | 1.880 | 242.9 | 33.1 | 50.9 | 55.87 | Pass |
| 1 | dedup-duplicate.pdf / balanced | 1.568 | 1.529 | 242.6 | 33.2 | 43.3 | 99.29 | Pass |
| 2 | pages-500.pdf / balanced | 2.190 | 2.186 | 245.9 | 78.2 | 1.0 | 54.79, 54.79 | Pass |
| 2 | pages-500-50mib.pdf / lossless | 1.994 | 1.854 | 451.9 | 39.1 | 199.3 | 0.66, 0.66 | Pass |
| 2 | pages-500-50mib.pdf / balanced | 13.490 | 13.324 | 891.1 | 89.4 | 535.2 | 0.66, 0.66 | Pass |
| 2 | pages-500-50mib.pdf / compact | 13.326 | 13.125 | 944.8 | 87.1 | 476.6 | 0.66, 0.66 | Pass |
| 2 | dedup-unique.pdf / balanced | 3.059 | 2.994 | 622.8 | 47.8 | 102.2 | 55.87, 55.87 | Pass |
| 2 | dedup-duplicate.pdf / balanced | 2.503 | 2.448 | 619.7 | 47.1 | 85.6 | 99.29, 99.29 | Pass |

Maximum sampled cgroup **944.8 MiB < 2048 MiB**; aggregate job disk
**535.2 MiB < 1200 MiB**; single job **273.3 MiB < 512 MiB**.
Всі positive requests завершилися в межах 120s; OOM/storage/time failures не спостерігались.

## Busy/cancel/cleanup та boundaries

| Перевірка | Факт |
|---|---|
| 501 pages, workers 1/2 | 422, cleanup Pass |
| 50MiB+1 byte, workers 1/2 | 413, cleanup Pass |
| Positive exact 500 pages/50MiB, all profiles, workers 1/2 | 6 successful batch cases, 9 successful max-size requests |
| Busy while heavy jobs active | 503 / Retry-After: 1; real e2e for both worker settings |
| Cancel heavy processing | Active GS observed for every worker, client context canceled; observed worker/GS PIDs reaped, zero jobs |
| Timeout and shutdown | Existing real e2e 504 at explicit tiny timeout; shutdown/disconnect upload and both tool groups Pass |
| Completion cleanup | 12 positive batch cases, all job roots empty after response |
| Container cleanup | No pdf-stage07 containers left; [evidence](../artifacts/stage-07/container-cleanup.txt) |

Max-load cancellation IDs: 1 worker **5554/5570**,
2 workers **5585/5617**, **5595/5618**. Before-uploads busy and qpdf/gs group cases
також є у [real-tests-accepted.log](../artifacts/stage-07/real-tests-accepted.log).
No fabricated runtime metrics; failed malformed fixture attempts залишено як Fail evidence.

## Deduplication cost

Actual `ghostscript.BuildArgs` експортує QA-only helper `quality-profiles`; лише
`-dDetectDuplicateImages=true/false` змінено в microbenchmark, production не змінено.
Три trials, alternating true/false order, 64 image objects / 8 pages;
ті самі pinned tools/2 CPU/2 GiB, весь CLI wall time. Raw calls/args/size/time:
[dedup.json](../artifacts/stage-07/dedup.json).

| Corpus | enabled median s | disabled median s | enabled bytes (median) | disabled bytes (median) |
|---|---:|---:|---:|---:|
| dedup-duplicate.pdf | 0.728 | 0.775 | 131103 | 8021720 |
| dedup-unique.pdf | 0.780 | 0.860 | 8025341 | 8025342 |

На цьому corpus enabled не показав time penalty, а на duplicates різко зменшив output.
Різницю кількох десятків ms не трактуємо як універсальне прискорення: мала вибірка,
shared host/cache noise. Dedup remains enabled; timeout/caps залишаються.

## Обмеження та відтворення

Це конкретні synthetic fixtures; worst-case 500 × 40MP scans, symbol/global JBIG2,
реальні документи/production server hardware та статистичний p95 **Not run**.
Немає заяви, що будь-який accepted-size PDF гарантовано успішний до timeout:
contract передбачає bounded error/cleanup. Sampler 20ms може пропустити короткий peak.

```sh
# Generate fixtures first with bundled QA Python, no DB or external data.
python3 testdata/pdf/generate_quality.py
make quality-load SECURITY_OPT= DOCKER=/snap/docker/current/bin/docker
```

Exact actual Docker command/image/binary SHA і audit —
[Stage 07 results](implementation-plans/pdf-compression-stages/results/07-quality-and-load.md).
