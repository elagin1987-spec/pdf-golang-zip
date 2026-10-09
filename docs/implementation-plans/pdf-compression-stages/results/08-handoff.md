# Результат Stage 08: Фінальна документація та передача

- Статус: **Completed**, локальний GS-v1 handoff, 2026-10-09.
- План до реалізації: [08-execution-plan.md](../08-execution-plan.md).
- Авторизація: користувач доручив взяти наступний stage, створити план і виконати його.
- Baseline: Stages 01–07 Completed; 125/125 source SHA Stage 07 збігаються до edits.
- Git: repository відсутній; commit/push/deploy не виконано.

## Фактичний результат

Зведено acceptance GS-v1, запуск із готового образу, точні профілі, caps, noauth/internal
API, warnings/refusals, codec/semantic/visual limits, shutdown/cleanup і rollback.
Виправлено застарілий поточний стан README/основного плану; історичні results збережено.
Production Go/tests/Dockerfile/profile flags/caps не змінено. Compose додано PDF_IMAGE override для відомого локального image ID (default той самий)
і виправлено застосування незмінного ліміту 2 CPU явними period/quota.

Готовий handoff: `artifacts/stage-08/{runtime-image.tar,source.tar.gz,evidence.tar.gz,SHA256SUMS}`.
Source contains code/docs/config/owned fixtures; evidence contains raw stages 01/06/07
and new Stage 08 inventories/smoke, preserving failed attempts. No .env/private docs.
Checksum/archive membership verified; image save/load and same-image restart rehearsed.
Package verification is recorded in `package-verification.json` after packaging.

## Точні змінені файли

- `README.md` — actual completion, operations/acceptance links.
- `docs/operations.md` — load/build/start/readiness/stop/cleanup/rollback, profiles,
  validation/fallback limits, runtime/license inventory and target-host follow-ups.
- `docs/acceptance-evidence.md` — nine overall criteria mapped to code/raw evidence.
- `compose.yaml`, `.env.example` — PDF_IMAGE override та explicit CPU period/quota, unchanged default runtime image.
- `docs/openapi.yaml` — final GS-v1 version label, endpoint behavior unchanged.
- `THIRD_PARTY_NOTICES.md` — final heading and server/worker dependency attribution.
- `docs/implementation-plans/2026-10-09-pdf-compression-service.md` — current state
  reconciled with completed runtime/quality/load/handoff; contract preserved.
- `docs/test-dependency-audit.md` — S08-D1, no unit run.
- stage `08-execution-plan.md`, `08-handoff.md`, `results/08-handoff.md`, tracker README,
  `decisions.md` — plan/status/results/operational decisions. Only Stage 08 tracker row changed.
- `artifacts/stage-08/` — ignored runtime/package/smoke evidence; no source secrets.

## Перевірки та докази

| Check | Actual result | Evidence |
|---|---|---|
| Stage 07 source/code baseline | Pass 125/125 SHA before changes | baseline-verification.json |
| Accepted quality/load evidence | Pass retained: 96 quality rows, 16 load rows; no rerun | stage-07 quality-accepted/quality.json, benchmark.json; evidence-verification.json |
| Tool/runtime identity | Pass gs 10.08.0, qpdf 12.4.2; all four hashes match Stage 07 | runtime-inventory.txt, runtime-image.json |
| Licenses/source/config inventory | Pass licenses/build configs/source archives present; Poppler/Python absent from runtime | runtime-inventory.txt, Dockerfile/notices review |
| Compose resolved isolation/limits | Pass UID 65532, init, readonly, caps, resources, tmpfs, image ID override | compose-resolved.yaml, container-inspect.json; local security=false explicitly |
| Ready/three positive profiles | Pass 200, each <= input; strict qpdf exit 0 | smoke-commands.json, *-headers.txt, {balanced,compact,lossless}.pdf |
| Special refusal | Pass 422 special_type_tagged | refusal.json, smoke-commands.json |
| Cleanup/shutdown | Pass no jobs, server_stopped, project containers removed | jobs-after-request.txt, shutdown.log, smoke-summary.json |
| Saved artifact rollback path | Pass docker save/load same image, ready/request/cleanup on restart | runtime-image.tar, smoke-commands.json, restart.pdf |
| Package hashes/members/docs links | Pass | SHA256SUMS, package-verification.json |
| Unit tests/dependency audit | Not run; no Go/test changes; S08-D1 reviewed scope | docs/test-dependency-audit.md; Stage 07 accepted race log retained |

New command execution: `/snap/docker/current/bin/docker` (Compose 2.3.3), Python3
orchestration of local checks, curl synthetic loopback. Full argv/exit/stdout/stderr
recorded in `smoke-commands.json`. Inventory: runtime container network none/readonly/
cap-drop ALL runs gs --version, qpdf --version, sha256sum and notices/source listing.
Smoke: `PORT=18088 BIND_IP=127.0.0.1 NO_NEW_PRIVILEGES=false PDF_IMAGE=sha256:bcc9...`
with dedicated `compose -p pdf-stage08-check up -d --no-build`, curls, qpdf --check,
`stop -t 10`, `down`, `save -o runtime-image.tar`, `load -i`, repeat readiness/request,
cleanup/down. No existing services changed. Exact executable script saved as
`artifacts/stage-08/smoke.py`; production profile unchanged.

Final runtime image:
`sha256:bcc9d6b9b8be30d147af2698cc56ab957d2db430f36656e1954784996da4c49f`.
server SHA `1cd267e8f08cfae277153577e338a1c4af6c5c726fea36089d61241395be676f`;
worker SHA `295b6bb3f3b028a0c8b84d87c1ad99f8de71c492f1f6625435feed9b312b54d9`.
GS/qpdf/tool/source hashes and base digests in inventory/Dockerfile.

## Критерії завершення

1. Stages 01–07 Completed and final code/reports consistent — source identity,
   retained quality/benchmark evidence, actual runtime inventory; all local criteria covered.
2. Every overall acceptance mapped to evidence — docs/acceptance-evidence.md; broad
   final checks supersede older small corpus/fake units, without rewriting historical claims.
3. Reproducible local artifact and startup examples — checksummed source/runtime/evidence,
   archive verification, loaded image ID, successful startup/shutdown/restart smoke.
4. Known codec/semantic/visual limits explicit — operations and quality report.

## Відхилення, обмеження та відкриті follow-ups

- Initial package check caught Compose CPU mismatch: cpus:2 resolved but actual
  NanoCpus/CpuQuota were zero. Changed to cpu_period=100000/cpu_quota=200000;
  final inspect and smoke pass. Previous inspect/smoke retained. Stage 07 docker-run
  benchmark had --cpus 2; Stage 06 Compose claim alone was insufficient for CPU proof.
- First smoke invocation exit 1: local Compose lacks --pull. No service started;
  corrected commands passed. `smoke-attempt1.json` retained, not Pass.
- Snap no-new-privileges+init exception unchanged. Default true on compatible target
  host **Not run**; operations explicitly preserves that acceptance action.
- Saved runtime notices are Stage 06 snapshot; final editorial notices in source archive.
  Tool binaries/licenses/source archives unchanged; no unnecessary rebuild/test rerun.
- Same known-image rollback rehearsed; different release upgrade/rollback **Not run**.
- No production deployment/target hardware/real documents/scanner/universal codec or
  throughput promise. Local scope complete; practical limits inherited from Stage 07.

## Handoff

All eight planned stages are locally Completed. No next implementation stage exists.
Public interfaces: POST /v1/compress, GET /healthz, GET /readyz; OpenAPI GS-v1.
Continue from this workspace or verified source package + runtime image + raw evidence.
Before rollout on a target host perform the exact protections/readiness/requests/cleanup
checks in operations; obtain separate deployment instruction. No service left running,
no DB interaction or migrations, no automatic commits/new chats.
