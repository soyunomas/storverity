# Changelog

All notable changes to StorVerity are documented in this file.

The project follows [Semantic Versioning 2.0.0](https://semver.org/) and uses Git tags in the form `vMAJOR.MINOR.PATCH` or `vMAJOR.MINOR.PATCH-PRERELEASE`.

## [Unreleased]

## [0.1.0-rc.1] - 2026-09-14

### Added

- Linux block-device discovery with nested mount, filesystem, system-disk and swap detection.
- Conservative machine-readable raw-test safety policy.
- Non-destructive filesystem verification with deterministic data, fsync, read-back verification, progress, cancellation and cleanup.
- Wails v2 + Svelte/TypeScript desktop application with live verification maps.
- Guarded raw capacity probe with distributed unique patterns, reverse-order verification, best-effort restoration and alias/wraparound detection.
- Short-lived single-use destructive confirmation challenges and post-open target revalidation.
- Human-readable text reports and versioned `storverity.report.v1` JSON reports.
- Native report export with private file permissions.
- Original StorVerity application icon and Linux desktop/AppStream metadata.
- x86_64 AppImage and deterministic Linux tarball packaging.
- SHA-256 release checksums, keyless Sigstore/cosign signing bundles and tagged GitHub Release automation.
- Keyboard-visible focus treatment, reduced-motion support and accessible status/error semantics.
- MIT license.
- Minimal root `storverity-helper` service for destructive raw probing over the system D-Bus.
- Per-operation polkit authorization for raw capacity probes without elevating the Wails/WebKit desktop process.
- Shared device identity/fingerprint helpers so the desktop and privileged service bind authorization to the same rediscoverable target metadata.
- D-Bus caller/session-scoped cancellation and automatic cancellation when the desktop caller disconnects.
- Hardened systemd unit plus D-Bus and polkit installation assets.
- Tarball helper installer and `make helper-build` / `make helper-install` development targets.

### Security

- Raw writes are denied for mounted, system, swap-containing, read-only, non-whole-disk and non-external targets.
- Linux raw devices are opened synchronously and exclusively, matched by `major:minor`, and revalidated again before the first write.
- The privileged helper independently repeats device discovery, raw-test safety, fingerprint, path, and Linux `major:minor` validation before destructive I/O.
- Device safety and identity are checked again after exclusive open and immediately before the first raw write, so a newly mounted, replaced, or reinserted target is rejected inside the root trust boundary.
- Invalid raw-probe geometry is rejected before polkit authorization or block-device open.
- The helper exposes only the high-level raw capacity probe rather than a generic privileged block-device read/write API.
- Release blobs are signed with the GitHub Actions OIDC identity and verified before publication; no long-lived signing key is stored in the repository.
- Ordinary CI never runs destructive tests against real block devices.

[Unreleased]: https://github.com/soyunomas/storverity/compare/v0.1.0-rc.1...HEAD
[0.1.0-rc.1]: https://github.com/soyunomas/storverity/releases/tag/v0.1.0-rc.1
