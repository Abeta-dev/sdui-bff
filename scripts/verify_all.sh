#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT_DIR"

echo "==> [1/5] Brand & machine-path hygiene"
./scripts/check_brand_hygiene.sh

echo "==> [2/5] Verifying gofmt formatting"
UNFORMATTED="$(gofmt -l .)"
if [ -n "$UNFORMATTED" ]; then
  echo "ERROR: Unformatted Go files detected:"
  echo "$UNFORMATTED"
  exit 1
fi
echo "PASSED: All Go files are gofmt-clean."

echo "==> [3/5] Running go vet ./..."
go vet ./...

echo "==> [4/5] Running go test -race -cover ./..."
go test -count=1 -race -cover ./...

echo "==> [5/5] Verifying static binary build (./cmd/sdui-bff)"
CGO_ENABLED=0 go build -o /tmp/abeta-sdui-bff-check ./cmd/sdui-bff && rm -f /tmp/abeta-sdui-bff-check

echo ""
echo "ALL VERIFICATION GATES PASSED for abeta-dev/sdui-bff."
