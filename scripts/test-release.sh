#!/usr/bin/env bash
# Smoke-test a published GitHub release: download, verify checksum, extract, run.
# Usage: ./scripts/test-release.sh [version]
# If version is omitted, the latest git tag is used.
set -euo pipefail

VERSION="${1:-$(git describe --tags --abbrev=0 2>/dev/null | sed 's/^v//')}"
if [[ -z "$VERSION" ]]; then
  echo "error: could not determine version; pass it explicitly: $0 <version>" >&2
  exit 1
fi

UNAME_OS=$(uname -s | tr '[:upper:]' '[:lower:]')
UNAME_ARCH=$(uname -m)

case "$UNAME_OS" in
  darwin) OS="darwin" ;;
  linux)  OS="linux"  ;;
  *) echo "error: unsupported OS: $UNAME_OS" >&2; exit 1 ;;
esac

case "$UNAME_ARCH" in
  x86_64|amd64) ARCH="amd64" ;;
  arm64|aarch64) ARCH="arm64" ;;
  *) echo "error: unsupported arch: $UNAME_ARCH" >&2; exit 1 ;;
esac

ARCHIVE="craft_${VERSION}_${OS}_${ARCH}.tar.gz"
BASE_URL="https://github.com/tcarcao/craft/releases/download/v${VERSION}"

WORK_DIR=$(mktemp -d)
trap 'rm -rf "$WORK_DIR"' EXIT

echo "==> testing release v${VERSION} (${OS}/${ARCH})"

echo "  fetching checksums.txt..."
curl -fsSL "${BASE_URL}/checksums.txt" -o "${WORK_DIR}/checksums.txt"

EXPECTED_HASH=$(grep "${ARCHIVE}" "${WORK_DIR}/checksums.txt" | awk '{print $1}')
if [[ -z "$EXPECTED_HASH" ]]; then
  echo "error: no checksum entry for ${ARCHIVE}" >&2
  exit 1
fi

echo "  downloading ${ARCHIVE}..."
curl -fsSL --progress-bar "${BASE_URL}/${ARCHIVE}" -o "${WORK_DIR}/${ARCHIVE}"

echo "  verifying checksum..."
if [[ "$OS" == "darwin" ]]; then
  ACTUAL_HASH=$(shasum -a 256 "${WORK_DIR}/${ARCHIVE}" | awk '{print $1}')
else
  ACTUAL_HASH=$(sha256sum "${WORK_DIR}/${ARCHIVE}" | awk '{print $1}')
fi

if [[ "$ACTUAL_HASH" != "$EXPECTED_HASH" ]]; then
  echo "error: checksum mismatch" >&2
  echo "  expected: $EXPECTED_HASH" >&2
  echo "  got:      $ACTUAL_HASH" >&2
  exit 1
fi
echo "  checksum OK"

echo "  extracting..."
tar xzf "${WORK_DIR}/${ARCHIVE}" -C "${WORK_DIR}" craft

if [[ "$OS" == "darwin" ]]; then
  xattr -dr com.apple.quarantine "${WORK_DIR}/craft"
fi

chmod +x "${WORK_DIR}/craft"

echo "  craft --version:"
"${WORK_DIR}/craft" --version

echo "  craft lsp --help (exit 0?)..."
"${WORK_DIR}/craft" lsp --help > /dev/null

echo ""
echo "==> release v${VERSION} OK (${OS}/${ARCH})"
