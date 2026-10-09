# GS-v1: запуск і передача

2026-10-09. Локальний функціональний/quality/load handoff завершено.
Цільовий production host/deploy не перевірявся. API внутрішній, без авторизації;
Compose слухає 127.0.0.1. Для внутрішньої мережі задайте BIND_IP адресою інтерфейсу.
Обробка локальна, без БД/Redis/зовнішніх API. Не запускайте міграції.

## Current courses support (2026-10-09)

Current source accepts up to 300 MiB and 1000 pages (the same parser/config cap),
with MAX_PAGES allowed to be lower. Compose passes 300/310 MiB upload/request,
2 GiB job, 4 GiB aggregate RAM, 3500 MiB jobs tmpfs and 512 MiB /tmp. Upload /
processing / write timeout defaults in Compose: 120s / 600s / 660s. The actual
server variable is PROCESSING_TIMEOUT; JOB_TIMEOUT is ignored.

ALLOW_SPECIAL_TYPES=true admits attachments/tagged/multimedia and the existing
relaxed forms/layers/JavaScript/annotation cases exclusively via qpdf lossless.
Special features and decoded embedded payloads are checked; no GS flattening is
attempted for these inputs. JavaScript remains in the document and is not executed.
Strict mode continues refusing them. Encrypted/signed/PDF-A/Launch refusal remains.
A preservation mismatch returns original bytes with special_features_changed;
mandatory tool failures and limits remain errors. See README for validation caps.

Rebuild the current source and use the resulting image tag/ID explicitly; merely
changing env on an older image does not add the new policy implementation.
The stage numbers, caps and strict policy descriptions below describe historical
acceptance builds and are not the current large-PDF configuration.

## Готовий пакет і відтворення

Stage 08 is the historical pre-fix package. For current policy/XMP/qpdf fixes,
build current source or use the local `pdf-compression:policy-validation` image;
see [current-build evidence](policy-validation-fixes-report.md). The archive-loading
example below intentionally restores the older release and does not include fixes.

`artifacts/stage-08/` містить `runtime-image.tar`, `source.tar.gz`, `evidence.tar.gz`,
`SHA256SUMS`, runtime inventory/image inspect та smoke evidence. Source містить
код, fixtures, конфігурацію і документацію; evidence — raw artifacts stages 01/06/07
та Stage 08 перевірки. Приватних документів і `.env` пакет не містить.

```sh
# У папці пакета; перевірити всі три архіви до використання.
sha256sum -c SHA256SUMS
mkdir source
tar -xzf source.tar.gz -C source
# Докази необов'язкові для runtime, але потрібні для acceptance review.
tar -xzf evidence.tar.gz -C source
docker load -i runtime-image.tar
cd source
cp .env.example .env
# Відомий локальний image ID; це ID образу, не registry RepoDigest.
export PDF_IMAGE=sha256:bcc9d6b9b8be30d147af2698cc56ab957d2db430f36656e1954784996da4c49f
docker image inspect "$PDF_IMAGE" >/dev/null
docker compose -p pdf-gsv1 up -d --no-build
curl --fail-with-body http://127.0.0.1:8080/readyz
curl --fail-with-body -D headers.txt -o compressed.pdf \
  -F file=@testdata/pdf/flate-rgb-600dpi.pdf -F profile=balanced \
  http://127.0.0.1:8080/v1/compress
```

На цьому snap Docker host init + no-new-privileges=true повертає operation not permitted.
Лише для відтворення локального smoke задайте `NO_NEW_PRIVILEGES=false` перед Compose.
Default лишається true; успіх цього default на сумісному цільовому host ще **Not run**.
Не трактуйте локальний виняток як production security acceptance.

Збірка з source: `docker compose build pdf` (потрібна мережа для pinned sources/APT).
Потім `PDF_IMAGE=pdf-compression:stage06 docker compose up -d --no-build`.
Local Compose 2.3.3 не підтримує `up --pull never`; спочатку перевіряється локальний
image ID через inspect, запуск не виконує build.
Base images locked by digest, CLI archives by SHA-256, APT snapshot 20261008T000000Z;
Go 1.27.2, Ubuntu 24.04, gs 10.08.0, qpdf 12.4.2. Dockerfile — джерело точних digests/hashes.
Повторна збірка не обіцяє тотожний image ID/байти; готовий image archive фіксує
саме перевірені binaries. Після нового build звірте inventory і виконайте affected QA.
Runtime archive зберігає notices snapshot Stage 06; фінальні редакційні notices є в source.
Ліцензії/CLI source archives/build configs у runtime залишаються тими самими.

## Профілі та межі гарантій

| Setting | balanced | compact | lossless |
|---|---|---|---|
| Engine | GS + qpdf candidates | GS + qpdf candidates | qpdf only |
| Color/Gray resolution | 300 dpi | 150 dpi | no image rewrite |
| Downsample type / threshold | Bicubic / 1.1 | Bicubic / 1.1 | — |
| AutoFilter color/gray | true | true | — |
| QFactor | 0.4 | 0.76 | — |
| Color H/V Samples | [1 1 1 1] | [2 1 1 2] | — |
| Gray H/V Samples | [1 1 1 1] | [1 1 1 1] | — |
| Mono downsample | false | false | — |

GS: PDF 1.7, AutoRotatePages=None, LeaveColorUnchanged, ConvertCMYKImagesToRGB=false,
JPEG/JPX passthrough enabled when applicable, EmbedAllFonts/SubsetFonts/CompressFonts,
CompressStreams, DetectDuplicateImages=true, FastWebView=false, PreserveAnnots and
PreserveAnnotTypes=[/Link], NeverEmbed=[]. Trusted settings via `-c`; SAFER and exact
input/output permit paths. No user PostScript/settings. Full argv source:
`internal/ghostscript/{profiles,settings}.go`. qpdf lossless recompresses Flate/streams,
no lossy image optimization; args are in `internal/qpdf/client.go`.

Strict qpdf input check precedes policy. Encrypted/signed/AcroForm/XFA/PDF-A/attachments/
portfolios/JavaScript/tagged/layers/non-Link annotations, Link appearances and unsupported
Link actions are refused. URI/GoTo links and supported bookmarks are checked.
CMYK/ICC/Separation/DeviceN skip GS (`complex_color_preserved`); unknown image filters
skip GS (`unsupported_image_filter`); oversized images skip GS (`image_pixel_limit`).

Candidates must pass qpdf validity and semantic snapshots: page count/order/physical boxes,
rotation/UserUnit, URI/GoTo and outlines (geometry tolerance 0.01 pt). GS also requires
normalized txtwrite text equality. Tool warnings/font substitution or unavailable text
comparison cause fallback. Runtime performs no visual rendering comparison.
`ghostscript_failed`, `ghostscript_warning`, `ghostscript_text_changed`,
`ghostscript_features_changed`, `text_validation_unavailable` report rejected GS paths.
Mandatory qpdf generation/validation failure returns an error, rather than success.
Policy also scans nested action dictionaries/arrays, including `/AA` and `/Next`,
with depth 32 and 100000 container-node limits; incomplete traversal returns 422.
Absent XMP is allowed, but unreadable declared `/Metadata` stops processing.
Missing/unlaunchable qpdf returns 503, internal tool/storage failure returns 500,
and JSON/object limits return 422. These failures also stop GS candidate validation;
they cannot be reported as successful fallback. A document-invalid GS candidate
or changed geometry/links/text still permits a validated lossless/original fallback.
The smallest valid candidate wins only if strictly smaller; otherwise exact original bytes.
Warnings can coexist with compressed qpdf/original output. No minimum savings guarantee.

GS streams/IDs/codecs/metadata need not be byte-identical. No OCR, full-page rasterization,
universal fidelity or every-codec-feature guarantee. Lossless exact rendered pixels,
mono decoded pixels, text/fonts/symbols/semantics passed the owned corpus. JPX decoder
is verified, JPX encoder not promised. JBIG2 immediate MMR region is verified;
symbol dictionaries/globals and real scanner/device acceptance are **Not run**.
UserUnit/rotation fixture takes truthful qpdf fallback. GS visual screening MAE 6/255
(and explicitly 8/255 for compact Indexed) plus manual review is fixture QA, not a
per-request guarantee. See quality report and acceptance matrix.

## Ресурси, readiness і діагностика

Env/defaults/caps and HTTP headers/errors: [README](../README.md), [OpenAPI](openapi.yaml).
Historical Compose passed only ENABLE_LOSSY, MAX_WORKERS and PROCESSING_TIMEOUT;
other server env settings need an explicit Compose override. BIND_IP/PORT/PDF_IMAGE/
NO_NEW_PRIVILEGES configure Compose, not server. `ENABLE_LOSSY=false` accepts only
explicit profile=lossless, requires qpdf but not GS. Do not change pinned tool versions
without repeating relevant acceptance.

Container: UID/GID 65532, init, read-only root, cap_drop ALL, default no-new-privileges,
2 GiB/2 CPU (cpu_period=100000, cpu_quota=200000)/128 PID, /tmp 256m and /work/jobs 1200m noexec/nosuid tmpfs. One jobs
root per server. 50 MiB file, 51 MiB multipart, 500 pages, 512 MiB job, 1/2 workers.
Worker GOMEMLIMIT=384MiB is a soft GC limit; RAM/tmpfs caps are aggregate. The 20ms
job-size watcher can overshoot briefly. Timeout=120s starts after upload, not receipt
of headers. Read header/upload defaults 10s/60s, write 180s refreshed after upload.

`/healthz` is liveness; `/readyz` checks exact tools, executable worker and writable jobs.
A 503 worker_busy with Retry-After=1 means no free slot; no queue/body acceptance.
Client retry is caller-owned. 422 is a policy/invalid/limit refusal, not retryable by
changing compression settings blindly. 500/503/504: retain request ID, inspect
`docker compose -p pdf-gsv1 logs --tail 100 pdf`, readiness and resource availability.
Logs contain status/code/bytes/time/engine/warnings, no PDF contents or raw tool stderr.
Do not share document bodies in diagnostics.

## Зупинка, cleanup і rollback

```sh
# Припинити нові запити від внутрішніх клієнтів перед зупинкою.
docker compose -p pdf-gsv1 stop -t 10 pdf
docker compose -p pdf-gsv1 down
```

SIGTERM stops admission/uploads, cancels worker/tool process groups TERM → grace → KILL,
waits and cleans jobs. Server shutdown budget 5s; Compose grace 10s. Tmpfs data disappears
with container removal. Startup removes only marked `pdfjob-*` in its dedicated root.
Do not point TEMP_DIR at a shared documents folder or delete arbitrary host directories.

Before replacing a release retain its image ID/archive, SHA256SUMS, source/env and evidence.
Rollback: stop current service, load the prior saved archive, set PDF_IMAGE to its verified
image ID, run the same `up --no-build`, verify readiness + one synthetic
request and cleanup. No migration/data restore is needed. Stage 08 rehearsed this
stop/load/same-known-image restart path, not an upgrade between different releases.

## Ліцензії та цільовий host acceptance

Runtime retains GS LICENSE/COPYING, qpdf LICENSE.txt, Go LICENSE, Ubuntu copyrights,
CLI source archives and build configs. Built-in GS ROM fonts/resources/CMaps/ICC and
fontconfig lookup are included; no private user font catalog is installed. Synthetic
DejaVu font license is in testdata. Poppler/Pillow/zbar/reportlab/pypdf are QA-only.
[Notices](../THIRD_PARTY_NOTICES.md) records actual software; open-source engine choice
and separate CLI invocation do not constitute a legal conclusion for integration.

Before internal rollout on the target host: verify default security option with init,
nonroot/read-only/tmpfs/resources, exact image/tool checksums, ready/positive/refusal
requests, cancel/cleanup and shutdown. Target hardware/real PDFs, universal throughput
and different-release rollback remain outside this local handoff. Local bounded benchmark
is 500 pages/50 MiB at 1/2 workers, not an SLA. No service is left running by Stage 08.

Compose 2.3.3 на цьому host ігнорував `cpus: 2`: inspect показав нульові NanoCpus/
CpuQuota. Stage 08 замінив shorthand на явні cpu_period=100000/cpu_quota=200000;
actual inspect тепер підтверджує 2 CPU. Stage 07 benchmark використовував docker run
--cpus 2, тому його CPU bounds залишаються чинними; старий Stage 06 Compose check
сам по собі не підтверджував CPU isolation.
