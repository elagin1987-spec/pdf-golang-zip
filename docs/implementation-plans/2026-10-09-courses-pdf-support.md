# Courses PDF support — implementation plan

Inputs: four original PDFs in /home/alex/Desktop/courses, 52.83–259.66 MiB,
43–552 pages. Baseline API evidence: results/courses-2026-10-09.

1. Preserve existing uncommitted work. No migrations, database writes or remote processing.
2. Set the qpdf parser hard page cap to 1000 and share that bound with config.
   Keep the configured lower MaxPages enforcement. Retain 300 MiB input, 310 MiB
   request, 2 GiB job bounds. Compose must pass all limits, PROCESSING_TIMEOUT=600s,
   WRITE_TIMEOUT=660s and READ_TIMEOUT=120s; never use unsupported JOB_TIMEOUT.
3. With AllowSpecialTypes=true allow attachment markers, tagged structure and
   multimedia encountered in the courses documents. Strict mode retains refusals.
   Encrypted PDFs, signatures, PDF/A and Launch actions remain outside this scope.
4. Mark accepted special structures as lossless-only. Skip Ghostscript for all
   allowed forms/layers/annotations/JavaScript/attachments/tagged features so qpdf
   preserves them. Replace claims of flattening/stripping with preserved warnings.
5. Validate qpdf output with existing integrity/page/link/bookmark checks, plus
   a reference-number-independent fingerprint of the reachable catalog graph,
   excluding stream encoding keys; compare decoded embedded-file payload hashes.
   Bound traversal and decoded reads. Do not accept lost special features.
6. Add boundary tests (1000/1001, configured lower pages), strict/relaxed policy
   cases, no Ghostscript for special structures, and preservation rejection tests.
   Before every test run review imports/helpers/init/config and record DB audit.
   Run tests with no network and private temporary storage in the pinned Go container.
7. Build a fresh uniquely tagged local runtime from the modified source. Start
   an isolated local API on a new loopback port, leaving the existing service alone.
8. Repeat balanced requests for all four original PDFs, retain every response,
   timings/request IDs/warnings/size savings, verify successful outputs with qpdf
   and pdfinfo. Confirm page counts and embedded-file preservation. Stop only
   the newly created API after processing; originals and previous evidence remain.
9. Actual-input finding: second-edition PDF has qpdf inflate warnings in source
   and rewrite. Preserve exact original after policy/limits rather than silently
   treating warning-bearing rewrites as valid compression; report unchanged and
   source_had_stream_warnings/compression_skipped_source_warnings.
10. Update README/OpenAPI/operations and deliver a before/after report with output
   links. Acceptance: all four return HTTP 200 with checked compressed or explicitly warning-bearing exact unchanged
   PDF <= original size; do not promise a minimum savings percentage.
