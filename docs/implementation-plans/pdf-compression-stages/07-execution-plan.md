# Stage 07 execution plan — 2026-10-09 / GS-v1

1. Confirm Stage 06 source interfaces and pinned tools; preserve all production caps.
2. Extend owned fixtures and a SHA-256 acceptance manifest: codecs/predictors, Cyrillic/embedded and unembedded fonts, OCR, vectors/small text/QR/Code128, masks/inline/nested Forms/shared placements, rotation/UserUnit, complex-color fallback and policy negatives.
3. Add real-tool integration matrix and API boundary tests. Independent Poppler QA compares per-page text (including form-feed/order), boxes/rotation, lossless renders, mono decoded pixels, fonts, known image dimensions and symbol decoding. Save PDFs, commands, JSON and render contact sheets. Inspect selected crops manually. Reject failures rather than treating not-run as pass.
4. Benchmark real HTTP with 1/2 workers at 500 pages and 50 MiB, measuring end-to-end/server processing, sampled cgroup RAM/temp disk, cleanup/busy/cancel. Compare GS dedup on/off with actual adapter arguments on duplicate and unique images. Record sample intervals and limitations.
5. Before each unit run inspect imports/helpers/init/TestMain/config and dependency closure, append audit; use local network-none test containers and synthetic data only. No database, migrations or external services.
6. Make only evidence-driven targeted fixes; rerun affected checks. Fill quality/benchmark/results reports and Stage 07 tracker row. Do not implement Stage 08.

Authorized by user: create the plan and execute it in this chat. No git repository/commit available.
