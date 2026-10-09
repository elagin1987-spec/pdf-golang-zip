# GS-v1 acceptance evidence — 2026-10-09

Local handoff completed. Stages 01–07 historical reports are retained; later Stage 06/07
checks cover the final runtime. Old fake-tool/host unit passes are not decoder or quality
proof. Stage 08 baseline-verification checks all 125 Stage 07 source hashes before docs edits.

Subsequent policy/validation fixes and their current-build checks are recorded in
[the fixes report](policy-validation-fixes-report.md). The original Stage 08 archives
remain a historical pre-fix release; do not load them expecting these fixes.

| Main contract §12 criterion | Evidence and scope |
|---|---|
| 1. Pinned tools/binaries/fonts/notices | Dockerfile base digests/source SHA/APT snapshot; Stage 01 results; stage-08/runtime-inventory.txt and runtime-image.json; runtime hashes equal Stage 07 accepted binaries |
| 2. Exact profile/SAFER/no preset | internal/ghostscript profiles/settings + Stage 04 report; Stage 07 actual quality-profiles export/raw commands; Stage 01 SAFER read/write denial report |
| 3. 600→300/150 dpi, no upscale, placement | stage-07/quality-accepted/quality.json + quality-commands.json + raw GS inventories/renders; UserUnit/Rotate case preserves features via explicit fallback |
| 4. Text/OCR/pages/vectors/links/bookmarks/visual QA | quality report; 69 independent-features snapshots, Poppler text/renders/fonts, zbar symbols and manual-review.md/equivalence; no per-request visual proof |
| 5. Real codec support/mono/fallback | quality report codec matrix; 12 returned + 8 raw GS exact mono decodes and mono-codec-crosscheck.json; CMYK/ICC/ASCIIHex truthful fallback; JBIG2 globals/symbol corpus not covered |
| 6. Valid, <=original, unchanged exact, no failed candidates | 69 successful quality rows + real-tests-accepted.log; Stage 06 e2e-final.log; mandatory qpdf errors tested; Stage 08 package smoke all profiles |
| 7. Special input/limits/cancel/cleanup/groups | Stage 06 e2e-final.log/storage/worker race logs; Stage 07 27 refusal rows, benchmark.json 16 cases, real-tests-accepted.log 500-page/50MiB/busy/cancel; Stage 08 shutdown/restart cleanup |
| 8. Unit audits/no DB/migrations/safe logs | docs/test-dependency-audit.md, Stage 06/07 dependency inventory/reviews and unit logs; production go.mod/source logging review; Stage 08 no unit run, synthetic smoke only |
| 9. Eight actual results and final artifact | results/01–08, tracker, operations; Stage 08 checksummed source/runtime/evidence archives and package-verification.json |

Persistent evidence roots: `artifacts/stage-01`, `artifacts/stage-06`,
`artifacts/stage-07`, `artifacts/stage-08`. They are ignored by Git but included in
handoff archives as described in operations. Failed Stage 07 attempts remain in
raw evidence; only `quality-accepted`/accepted test logs establish acceptance.

96/96 quality rows (69 PDF + 27 refusal), 69 independent semantic snapshots,
23 exact lossless render sets, 16/16 load cases are the accepted Stage 07 counts.
Source fixtures/provenance/SHA: testdata/pdf/manifest.json and quality-manifest.json.
[Quality report](quality-report.md), [benchmark report](benchmark-report.md),
[operations](operations.md) define practical limits.

Default no-new-privileges + init on compatible host, production hardware/real scanner
PDFs, every JBIG2 feature, statistical throughput and production deployment are **Not run**.
Local snap compatibility override is explicit; these are retained operational follow-ups,
not invented Pass or blockers for the agreed local handoff scope.
