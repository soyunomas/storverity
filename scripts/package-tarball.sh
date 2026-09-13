#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
VERSION="${VERSION:-0.1.0-dev}"
ARCH="${ARCH:-x86_64}"
BINARY="${BINARY:-$ROOT/build/bin/storverity}"
DIST_DIR="${DIST_DIR:-$ROOT/dist}"
WORK_DIR="${TARBALL_WORK_DIR:-$ROOT/build/tarball}"
SOURCE_DATE_EPOCH="${SOURCE_DATE_EPOCH:-0}"

if [[ "$ARCH" != "x86_64" ]]; then
  echo "error: release tarball currently supports x86_64 only" >&2
  exit 2
fi
if [[ ! -x "$BINARY" ]]; then
  echo "error: desktop binary not found or not executable: $BINARY" >&2
  exit 2
fi

NAME="StorVerity-${VERSION}-linux-${ARCH}"
STAGE="$WORK_DIR/$NAME"
OUTPUT="$DIST_DIR/$NAME.tar.gz"
rm -rf "$WORK_DIR"
mkdir -p "$STAGE/bin" "$STAGE/share/applications" "$STAGE/share/icons" "$STAGE/share/metainfo" "$DIST_DIR"

install -m 0755 "$BINARY" "$STAGE/bin/storverity"
install -m 0644 "$ROOT/packaging/linux/io.github.soyunomas.StorVerity.desktop" "$STAGE/share/applications/io.github.soyunomas.StorVerity.desktop"
install -m 0644 "$ROOT/assets/storverity.svg" "$STAGE/share/icons/io.github.soyunomas.StorVerity.svg"
install -m 0644 "$ROOT/packaging/linux/io.github.soyunomas.StorVerity.metainfo.xml" "$STAGE/share/metainfo/io.github.soyunomas.StorVerity.metainfo.xml"
install -m 0644 "$ROOT/README.md" "$STAGE/README.md"
install -m 0644 "$ROOT/LICENSE" "$STAGE/LICENSE"

rm -f "$OUTPUT"
TZ=UTC tar \
  --sort=name \
  --mtime="@$SOURCE_DATE_EPOCH" \
  --owner=0 --group=0 --numeric-owner \
  --format=gnu \
  -C "$WORK_DIR" \
  -czf "$OUTPUT" "$NAME"

echo "$OUTPUT"
