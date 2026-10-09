#!/bin/sh
# Inventory only. A manual dependency review is required before EVERY unit run.
set -eu
cd "$(dirname "$0")/.."
printf '%s\n' 'Test and module inventory (manual review required):'
find . -path './artifacts' -prune -o -name '*_test.go' -print
cat go.mod
if test -f go.sum; then cat go.sum; fi
printf '%s\n' 'Imports, initialization, helpers, configuration, and suspicious interactions:'
find cmd internal tests -name '*.go' -type f -exec sed -n '/^import /,/^)/p' {} \; 2>/dev/null || true
if command -v rg >/dev/null; then
    rg -n 'TestMain|func init\(|database/sql|sql\.Open|gorm|pgx|redis|mongodb|net/http|net\.Dial|os\.Getenv|exec\.Command|WriteFile|RemoveAll' cmd internal tests go.mod go.sum 2>/dev/null || true
fi
printf '%s\n' 'This script does not certify absence of DB interaction. Read all dependencies before running unit tests.'
