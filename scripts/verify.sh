#!/usr/bin/env bash
# SPDX-License-Identifier: Apache-2.0
#
# Verification script: compares Go and Python dd-license-attribution outputs.
#
# Usage:
#   ./scripts/verify.sh <repo-url> [github-token]
#
# Examples:
#   ./scripts/verify.sh https://github.com/DataDog/dd-license-attribution "$GITHUB_TOKEN"
#   make verify REPO=https://github.com/DataDog/chaos-controller TOKEN="$GITHUB_TOKEN"
#
# Prerequisites:
#   - Go binary built: make build
#   - Python version installed: pip install dd-license-attribution
#   - Both must be on PATH or in expected locations

set -euo pipefail

REPO="${1:?Usage: verify.sh <repo-url> [github-token]}"
TOKEN="${2:-${GITHUB_TOKEN:-}}"

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_DIR="$(dirname "$SCRIPT_DIR")"
GO_BINARY="${PROJECT_DIR}/bin/dd-license-attribution"
TMPDIR="$(mktemp -d)"

cleanup() {
    rm -rf "$TMPDIR"
}
trap cleanup EXIT

# Build Go binary if not present
if [ ! -f "$GO_BINARY" ]; then
    echo "Building Go binary..."
    make -C "$PROJECT_DIR" build
fi

# Common flags
FLAGS=""
if [ -n "$TOKEN" ]; then
    FLAGS="--github-token $TOKEN"
fi
# Use --no-scancode-strategy for comparison since the Go version uses
# go-license-detector instead of scancode (results may differ).
FLAGS="$FLAGS --no-scancode-strategy"

echo "=== Verifying against: $REPO ==="
echo ""

# Run Python version
echo "Running Python version..."
if command -v dd-license-attribution &>/dev/null; then
    dd-license-attribution generate-sbom-csv "$REPO" $FLAGS > "$TMPDIR/py_out.csv" 2>/dev/null || {
        echo "WARNING: Python version failed, skipping comparison"
        exit 0
    }
else
    echo "WARNING: Python dd-license-attribution not found on PATH. Skipping Python comparison."
    echo "Install with: pip install dd-license-attribution"
    exit 0
fi

# Run Go version
echo "Running Go version..."
"$GO_BINARY" generate-sbom-csv "$REPO" $FLAGS > "$TMPDIR/go_out.csv" 2>/dev/null

# Sort both outputs (skip header)
(head -1 "$TMPDIR/py_out.csv" && tail -n+2 "$TMPDIR/py_out.csv" | sort) > "$TMPDIR/py_sorted.csv"
(head -1 "$TMPDIR/go_out.csv" && tail -n+2 "$TMPDIR/go_out.csv" | sort) > "$TMPDIR/go_sorted.csv"

# Compare
echo ""
echo "--- Results ---"
echo "Python rows: $(wc -l < "$TMPDIR/py_sorted.csv")"
echo "Go rows:     $(wc -l < "$TMPDIR/go_sorted.csv")"
echo ""

if diff -u "$TMPDIR/py_sorted.csv" "$TMPDIR/go_sorted.csv" > "$TMPDIR/diff.txt" 2>&1; then
    echo "PASS: Outputs are identical!"
else
    DIFF_LINES=$(wc -l < "$TMPDIR/diff.txt")
    echo "DIFF: Outputs differ ($DIFF_LINES lines in diff)"
    echo ""
    echo "First 50 lines of diff:"
    head -50 "$TMPDIR/diff.txt"
    echo ""
    echo "Full diff saved to: $TMPDIR/diff.txt"
    # Copy diff to project dir for inspection
    cp "$TMPDIR/diff.txt" "$PROJECT_DIR/verify-diff.txt"
    echo "Also saved to: $PROJECT_DIR/verify-diff.txt"
fi
