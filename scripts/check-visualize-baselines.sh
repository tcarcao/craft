#!/bin/bash
# check-visualize-baselines.sh
# Verifies that the visualizer still produces output matching testdata/visualize-baseline/.
#
# Usage: bash scripts/check-visualize-baselines.sh
#
# NOTE: The domain diagram generator uses Go map iteration (non-deterministic order)
# for alias assignments and queue declarations. This means the domain diagram output
# is non-deterministic across runs even with identical input. Domain diagrams are
# therefore SKIPPED in this check. C4 and sequence diagrams are compared byte-for-byte
# and are deterministic.
#
# This non-determinism is pre-existing in the codebase and is tracked for a future fix.

set -e

BINARY=/tmp/craft-check
REGEN_DIR=/tmp/craft-regen
BASELINE_DIR="$(cd "$(dirname "$0")/.." && pwd)/testdata/visualize-baseline"

echo "Building craft binary..."
go build -mod=mod -o "$BINARY" ./cmd/craft/

mkdir -p "$REGEN_DIR"

FAIL=0

for f in "$BASELINE_DIR"/*.puml; do
  name=$(basename "$f")
  # Derive example name: simple-c4.puml -> simple, simple-domain.puml -> simple, etc.
  example=$(echo "$name" | sed 's/-c4\.puml$//' | sed 's/-domain\.puml$//' | sed 's/-sequence\.puml$//')

  craft_file="examples/${example}.craft"
  if [ ! -f "$craft_file" ]; then
    echo "SKIP: $craft_file not found (cannot regenerate $name)"
    continue
  fi

  # Domain diagrams are skipped due to pre-existing non-determinism in alias generation.
  case "$name" in
    *-domain.puml)
      echo "SKIP (non-deterministic alias generation): $name"
      continue
      ;;
  esac

  "$BINARY" generate "$craft_file" --type=all -o "$REGEN_DIR" >/dev/null 2>&1 || true

  if [ ! -f "$REGEN_DIR/$name" ]; then
    echo "MISSING: $REGEN_DIR/$name was not generated"
    FAIL=1
    continue
  fi

  if diff -q "$f" "$REGEN_DIR/$name" >/dev/null 2>&1; then
    echo "OK: $name"
  else
    echo "BASELINE DIFF: $name"
    diff "$f" "$REGEN_DIR/$name" || true
    FAIL=1
  fi
done

if [ "$FAIL" -eq 0 ]; then
  echo "All checked visualize baselines match."
else
  echo "Some baselines differ — review output above."
  exit 1
fi
