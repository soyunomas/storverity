# Development roadmap

## Phase 0 — Foundation — COMPLETE

Acceptance criteria:

- Go module and repository conventions exist.
- Architecture and destructive-I/O safety constraints are documented.
- Local `make check` runs formatting, vetting, and tests.
- GitHub Actions runs the same checks on pushes and pull requests.

## Phase 1 — Linux discovery and safety — COMPLETE

Acceptance criteria:

- Whole disks are discovered without writing to storage.
- Nested mounts/filesystems are aggregated recursively.
- Critical system mounts are detected through nested device stacks.
- Swap presence is surfaced.
- Raw-test eligibility returns machine-readable deny/warning reasons.
- Unit tests cover normal USB media, system disks, nested encrypted/LVM roots, mounted media, read-only media, and malformed `lsblk` data.

## Phase 2 — Non-destructive filesystem verification — COMPLETE

Acceptance criteria:

- Verification operates inside a selected mounted filesystem, not the raw block device.
- Test data is deterministic per region and detects corruption/misdirection.
- Writes are flushed before read-back verification.
- The engine exposes progress and cancellation.
- Temporary test data is cleaned up after success, cancellation, and ordinary failures.
- Tests use temporary directories and injected faults; no real USB device is required for CI.

## Phase 3 — Desktop UI — COMPLETE

Acceptance criteria:

- Wails v2 + Svelte/TypeScript application shell.
- Device cards with model, capacity, transport, mount state, and safety state.
- Live region map with pending/valid/corrupt/read-error/write-error states.
- Explicit confirmation flows for operations that may modify storage.

## Phase 4 — Raw destructive capacity probe — COMPLETE

Acceptance criteria:

- Requires a fresh Phase 1 safety decision immediately before opening the device.
- Rejects mounted/system/swap/read-only devices.
- Samples regions across the advertised address space and detects alias/wraparound behavior.
- Restores sampled bytes where the selected algorithm can do so safely; otherwise the mode is explicitly destructive.
- Hardware-in-the-loop tests are documented separately from CI tests.

## Phase 5 — Productization — COMPLETE

Acceptance criteria:

- Human-readable and versioned machine-readable reports.
- AppImage plus documented distro/Flatpak packaging strategy.
- Tagged releases with changelog, SHA-256 manifests, and keyless Sigstore-signed release artifacts.
- Embedded version/commit/build metadata and deterministic tarball packaging.
- Open-source license and original application identity/assets.
- Accessibility and destructive-operation UX review with keyboard focus, reduced-motion and status/error semantics implemented in the desktop UI.

Exit state: ordinary CI validates Go/report schema tests, frontend checks, native Wails build, AppImage/tarball packaging and checksums; the tag-driven release workflow additionally signs/verifies release blobs before publishing.
