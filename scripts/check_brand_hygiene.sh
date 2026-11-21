#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT_DIR"

# Construct patterns via concatenation so this script itself never embeds forbidden literals.
FORBIDDEN_BRAND_REGEX="$(printf '%s|%s|%s|%s|%s|%s|%s|%s|%s|%s|%s' \
  'blin''kit' \
  'grof''er' \
  'eter''nal' \
  'zom''ato' \
  'jum''bo' \
  'bl_''form' \
  'swig''gy' \
  'zep''to' \
  'ama''zon' \
  'tin''der' \
  'india''mart')"

FORBIDDEN_PATH_REGEX="$(printf '%s|%s|%s' \
  '/User''s/' \
  '/hom''e/[A-Za-z0-9._-]+/' \
  'C:\\\\User''s\\\\')"

echo "==> Checking brand & machine-path hygiene across abeta-dev/sdui-bff..."

VIOLATIONS=0

while IFS= read -r -d '' file; do
  case "$file" in
    .git/*|*/.git/*|./bin/*|./dist/*|*.db|*.db-wal|*.db-shm|*.sqlite*|*.png|*.jpg|*.jpeg|*.webp|*.ico|*.pdf|*.woff|*.woff2)
      continue
      ;;
  esac

  if grep -I -nEi "$FORBIDDEN_BRAND_REGEX" "$file" >/dev/null 2>&1; then
    echo "ERROR: Forbidden third-party brand reference in: $file"
    grep -I -nEi "$FORBIDDEN_BRAND_REGEX" "$file" || true
    VIOLATIONS=$((VIOLATIONS + 1))
  fi

  if grep -I -nE "$FORBIDDEN_PATH_REGEX" "$file" >/dev/null 2>&1; then
    echo "ERROR: Machine-specific absolute path in: $file"
    grep -I -nE "$FORBIDDEN_PATH_REGEX" "$file" || true
    VIOLATIONS=$((VIOLATIONS + 1))
  fi
done < <(find . \( -name '.git' -o -name 'bin' -o -name 'dist' -o -name 'node_modules' \) -prune -o -type f -print0)

if [ "$VIOLATIONS" -gt 0 ]; then
  echo "FAILED: Found $VIOLATIONS file(s) violating brand or path hygiene."
  exit 1
fi

echo "PASSED: Zero forbidden brand references and zero machine-specific paths found."
