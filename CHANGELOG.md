# Changelog

All notable changes to StorVerity are documented in this file.

The project follows [Semantic Versioning 2.0.0](https://semver.org/) and uses Git tags in the form `vMAJOR.MINOR.PATCH`.

## [Unreleased]

## [0.1.0] - 2026-09-13

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
- SHA-256 release checksums and tagged GitHub Release automation.
- MIT license.

### Security

- Raw writes are denied for mounted, system, swap-containing, read-only, non-whole-disk and non-external targets.
- Linux raw devices are opened synchronously and exclusively, matched by `major:minor`, and revalidated again before the first write.
- Ordinary CI never runs destructive tests against real block devices.

[Unreleased]: https://github.com/soyunomas/storverity/compare/v0.1.0...HEAD
[0.1.0]: https://github.com/soyunomas/storverity/releases/tag/v0.1.0
