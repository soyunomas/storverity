#!/usr/bin/env bash
set -euo pipefail

if [[ "${EUID:-$(id -u)}" -ne 0 ]]; then
  echo "error: install-helper.sh must be run as root" >&2
  exit 2
fi

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
if [[ -x "$SCRIPT_DIR/libexec/storverity-helper" ]]; then
  SOURCE_ROOT="$SCRIPT_DIR"
  HELPER="$SOURCE_ROOT/libexec/storverity-helper"
  POLICY="$SOURCE_ROOT/share/polkit-1/actions/io.github.soyunomas.StorVerity.policy"
  DBUS_SERVICE="$SOURCE_ROOT/share/dbus-1/system-services/io.github.soyunomas.StorVerity.Helper1.service"
  DBUS_CONF="$SOURCE_ROOT/share/dbus-1/system.d/io.github.soyunomas.StorVerity.Helper1.conf"
  SYSTEMD_UNIT="$SOURCE_ROOT/lib/systemd/system/io.github.soyunomas.StorVerity.Helper1.service"
else
  SOURCE_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
  HELPER="$SOURCE_ROOT/build/bin/storverity-helper"
  POLICY="$SOURCE_ROOT/packaging/linux/polkit/io.github.soyunomas.StorVerity.policy"
  DBUS_SERVICE="$SOURCE_ROOT/packaging/linux/dbus/io.github.soyunomas.StorVerity.Helper1.service"
  DBUS_CONF="$SOURCE_ROOT/packaging/linux/dbus/io.github.soyunomas.StorVerity.Helper1.conf"
  SYSTEMD_UNIT="$SOURCE_ROOT/packaging/linux/systemd/io.github.soyunomas.StorVerity.Helper1.service"
fi

for file in "$HELPER" "$POLICY" "$DBUS_SERVICE" "$DBUS_CONF" "$SYSTEMD_UNIT"; do
  if [[ ! -f "$file" ]]; then
    echo "error: required helper asset is missing: $file" >&2
    exit 2
  fi
done

install -d -m 0755 \
  /usr/libexec \
  /usr/share/polkit-1/actions \
  /usr/share/dbus-1/system-services \
  /usr/share/dbus-1/system.d \
  /usr/lib/systemd/system

install -m 0755 "$HELPER" /usr/libexec/storverity-helper
install -m 0644 "$POLICY" /usr/share/polkit-1/actions/io.github.soyunomas.StorVerity.policy
install -m 0644 "$DBUS_SERVICE" /usr/share/dbus-1/system-services/io.github.soyunomas.StorVerity.Helper1.service
install -m 0644 "$DBUS_CONF" /usr/share/dbus-1/system.d/io.github.soyunomas.StorVerity.Helper1.conf
install -m 0644 "$SYSTEMD_UNIT" /usr/lib/systemd/system/io.github.soyunomas.StorVerity.Helper1.service

if command -v systemctl >/dev/null 2>&1; then
  systemctl daemon-reload
  systemctl stop io.github.soyunomas.StorVerity.Helper1.service >/dev/null 2>&1 || true
fi

echo "StorVerity privileged helper installed."
echo "The service is DBus-activated and will start on the next raw probe request."
