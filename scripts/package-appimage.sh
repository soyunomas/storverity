#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
VERSION="${VERSION:-0.1.0-dev}"
ARCH="${ARCH:-x86_64}"
BINARY="${BINARY:-$ROOT/build/bin/storverity}"
DIST_DIR="${DIST_DIR:-$ROOT/dist}"
CACHE_DIR="${APPIMAGE_TOOL_CACHE:-$ROOT/.cache/appimage-tools}"
WORK_DIR="${APPIMAGE_WORK_DIR:-$ROOT/build/appimage}"

if [[ "$ARCH" != "x86_64" ]]; then
  echo "error: AppImage packaging currently supports x86_64 only" >&2
  exit 2
fi
if [[ ! -x "$BINARY" ]]; then
  echo "error: desktop binary not found or not executable: $BINARY" >&2
  exit 2
fi

APPIMAGETOOL_URL="https://github.com/AppImage/appimagetool/releases/download/continuous/appimagetool-x86_64.AppImage"
APPIMAGETOOL_SHA256="a6d71e2b6cd66f8e8d16c37ad164658985e0cf5fcaa950c90a482890cb9d13e0"
RUNTIME_URL="https://github.com/AppImage/type2-runtime/releases/download/continuous/runtime-x86_64"
RUNTIME_SHA256="1cc49bcf1e2ccd593c379adb17c9f85a36d619088296504de95b1d06215aebbf"

mkdir -p "$CACHE_DIR" "$DIST_DIR"

download_verified() {
  local url="$1" expected="$2" dest="$3"
  if [[ -f "$dest" ]] && echo "$expected  $dest" | sha256sum --check --status; then
    return 0
  fi
  rm -f "$dest"
  curl --fail --location --retry 3 --retry-delay 2 "$url" --output "$dest"
  echo "$expected  $dest" | sha256sum --check --status || {
    echo "error: checksum mismatch for $dest" >&2
    rm -f "$dest"
    exit 3
  }
}

TOOL="$CACHE_DIR/appimagetool-x86_64.AppImage"
RUNTIME="$CACHE_DIR/runtime-x86_64"
download_verified "$APPIMAGETOOL_URL" "$APPIMAGETOOL_SHA256" "$TOOL"
download_verified "$RUNTIME_URL" "$RUNTIME_SHA256" "$RUNTIME"
chmod +x "$TOOL" "$RUNTIME"

rm -rf "$WORK_DIR"
APPDIR="$WORK_DIR/StorVerity.AppDir"
mkdir -p \
  "$APPDIR/usr/bin" \
  "$APPDIR/usr/share/applications" \
  "$APPDIR/usr/share/icons/hicolor/scalable/apps" \
  "$APPDIR/usr/share/metainfo"

install -m 0755 "$BINARY" "$APPDIR/usr/bin/storverity"
install -m 0755 "$ROOT/packaging/appimage/AppRun" "$APPDIR/AppRun"
install -m 0644 "$ROOT/packaging/linux/io.github.soyunomas.StorVerity.desktop" "$APPDIR/io.github.soyunomas.StorVerity.desktop"
install -m 0644 "$ROOT/packaging/linux/io.github.soyunomas.StorVerity.desktop" "$APPDIR/usr/share/applications/io.github.soyunomas.StorVerity.desktop"
install -m 0644 "$ROOT/assets/storverity.svg" "$APPDIR/io.github.soyunomas.StorVerity.svg"
install -m 0644 "$ROOT/assets/storverity.svg" "$APPDIR/usr/share/icons/hicolor/scalable/apps/io.github.soyunomas.StorVerity.svg"
install -m 0644 "$ROOT/packaging/linux/io.github.soyunomas.StorVerity.metainfo.xml" "$APPDIR/usr/share/metainfo/io.github.soyunomas.StorVerity.metainfo.xml"

OUTPUT="$DIST_DIR/StorVerity-${VERSION}-x86_64.AppImage"
rm -f "$OUTPUT"
APPIMAGE_EXTRACT_AND_RUN=1 ARCH=x86_64 VERSION="$VERSION" \
  "$TOOL" --runtime-file "$RUNTIME" "$APPDIR" "$OUTPUT"
chmod +x "$OUTPUT"

echo "$OUTPUT"
