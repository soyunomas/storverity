# Privileged raw-I/O helper

StorVerity keeps the Wails/WebKit desktop process unprivileged. Destructive raw probing is delegated to a minimal root helper over the system D-Bus, with per-operation authorization through polkit.

## Boundary

The desktop never asks the helper to read or write an arbitrary path. The helper exposes one high-level operation: run the StorVerity raw capacity probe for a rediscovered device that still satisfies the safety policy.

D-Bus contract:

- Service: `io.github.soyunomas.StorVerity.Helper1`
- Object: `/io/github/soyunomas/StorVerity/Helper1`
- Interface: `io.github.soyunomas.StorVerity.Helper1`
- Raw-probe authorization action: `io.github.soyunomas.StorVerity.raw-probe`
- Progress is emitted as a session-scoped D-Bus signal.
- Cancellation is accepted only from the D-Bus caller that owns the active session.

The wire payload is internal version-1 implementation detail and is encoded as JSON strings over D-Bus. It is not a public compatibility API.

## Authorization and safety sequence

The desktop first runs the normal short-lived, one-use confirmation flow. After the user types the exact destructive confirmation phrase, the desktop refreshes the selected device and binds the request to a stable device ID plus a fingerprint containing path, kernel name, Linux `major:minor`, serial, advertised size, vendor, model, and transport.

The root helper then performs its own checks instead of trusting the desktop decision:

1. Validate the request/session shape and bounded probe geometry.
2. Ask polkit to authorize the calling system-bus name for `io.github.soyunomas.StorVerity.raw-probe`.
3. Rediscover the device from `lsblk` using the stable device ID.
4. Re-run the raw-test safety policy and reject mounted, system, swap-containing, read-only, non-whole-disk, or non-external media.
5. Require the freshly computed fingerprint to match the fingerprint confirmed by the desktop.
6. Open only the rediscovered path with the existing exclusive/synchronous Linux block-device opener and verify the opened descriptor's `major:minor`.
7. Rediscover the same device again after open and repeat safety, fingerprint, path, and `major:minor` checks before the first write.
8. Generate the raw pattern seed inside the privileged helper and run the normal raw-probe engine.

This duplicates the safety decision across the privilege boundary intentionally. Compromising or confusing the UI must not turn a free-form `/dev/...` path into root raw I/O.

## Cancellation and client failure

Each run has a cryptographically random session ID and one D-Bus caller owner. The desktop sends `CancelRawProbe` when its context is cancelled. The helper also watches `org.freedesktop.DBus.NameOwnerChanged`; if the desktop connection disappears, the helper cancels the active run so the probe proceeds to the engine's best-effort restoration path instead of continuing orphaned.

Cancellation does not make raw probing non-destructive. A disconnect, power failure, fraudulent controller, media error, or restore failure can still leave sampled blocks damaged.

## Installation

The deterministic tarball contains the helper binary and system integration files. From an extracted release tarball, install the privileged component with:

```bash
sudo ./install-helper.sh
```

The installer places:

- `/usr/libexec/storverity-helper`
- the polkit action under `/usr/share/polkit-1/actions/`
- the D-Bus activation file under `/usr/share/dbus-1/system-services/`
- the D-Bus ownership policy under `/usr/share/dbus-1/system.d/`
- the hardened systemd unit under `/usr/lib/systemd/system/`

The helper is D-Bus activated; it is not intended to run continuously.

The AppImage remains an unprivileged application artifact and deliberately does not self-install root files. Raw probing from the AppImage therefore requires the matching helper to have been installed separately. Filesystem verification does not require the helper.

## systemd hardening

The packaged service runs as root but enables `NoNewPrivileges`, a private temporary directory, read-only system protection, home protection, kernel/control-group protection, `AF_UNIX`-only networking, personality locking, W^X-style memory restrictions, realtime restrictions, and a restrictive umask.

`PrivateDevices` and a closed `DevicePolicy` are intentionally not enabled because the helper must open the user-authorized removable block device. Access is constrained in application logic by fresh discovery, safety checks, fingerprint matching, descriptor identity, and exclusive open rather than by a blanket udev grant.

## Validation

Ordinary CI never writes to a real block device. It unit-tests the helper boundary with fake device discovery, authorization, media openers, and probe engines, including unauthorized callers, stale fingerprints, mounts appearing after open, cancellation ownership, and invalid geometry. CI also compiles the real D-Bus/polkit transport and packages the helper with the desktop artifacts.

A release candidate should additionally be tested manually on a supported Linux desktop with a sacrificial USB device:

1. Install the helper from the matching build.
2. Launch StorVerity as a normal user, not with `sudo`.
3. Prepare a raw probe against expendable, unmounted external media.
4. Confirm that the desktop requests polkit authorization and that declining it performs no raw probe.
5. Authorize the action and verify live progress/cancellation.
6. While testing separate sacrificial runs, verify that mounting the device during the pre-write window or replacing/reinserting it causes the run to abort rather than write to the changed target.
7. Confirm the Wails process remains owned by the normal user and only `storverity-helper` runs as root.

Manual polkit-agent behavior varies by desktop environment, so the interactive prompt itself is intentionally a hardware/desktop integration check rather than an ordinary headless CI test.
