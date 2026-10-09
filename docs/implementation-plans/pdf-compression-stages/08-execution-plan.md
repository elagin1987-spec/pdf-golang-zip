# Stage 08 execution plan — 2026-10-09 / GS-v1

Authorized by user: take the next stage, create its implementation plan, execute it.

1. Reconcile Stage 01–07 results with actual code/config and persistent artifacts; capture local runtime image identity, tool/binary/source checksums and bundled notices.
2. Update README and stale current-state paragraphs in the main contract. Write operations instructions for internal noauth startup/readiness, profiles/limits, shutdown/cleanup, immutable-image rollback and reproduction. Preserve the snap Docker security-option deviation explicitly.
3. Map every overall acceptance criterion to saved evidence, describe codec/semantic/visual limits, and create a checksum-verified local handoff archive with runtime image, source/config/docs and final inventory.
4. Run only new documentation/package/startup smoke checks needed by this stage, with synthetic PDFs. Do not rerun passed unit/quality/load tests without changed code or unresolved evidence. Before any unit run audit its dependencies; never run migrations or access a DB.
5. Record actual results/artifacts/deviations and local acceptance; update Stage 08 only in tracker. No deployment, commit, push or automatic chat creation.

Scope: documentation, env/notices and packaging evidence; minimal Docker/Compose/Makefile/OpenAPI consistency fixes only if demonstrated necessary. No production behavior/profile/cap changes planned.
