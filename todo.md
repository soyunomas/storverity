# StorVerity implementation plan

This file is the execution checklist for the project. A phase is only marked complete after its code has been formatted, vetted, tested locally where dependencies permit, and the corresponding GitHub Actions integration run is green.

## Engineering rules

- Keep storage algorithms in normal Go packages, independent from Wails/Svelte.
- Treat device discovery as read-only.
- Deny destructive access by default; never trust a free-form `/dev/...` path from the UI.
- Re-evaluate device identity and safety immediately before destructive I/O.
- No raw destructive operation is enabled until Phase 4.
- Every storage-writing code path must support cancellation and deterministic cleanup where possible.
- Add tests before publishing a phase. Prefer fault injection and temporary directories over real hardware in CI.
- Keep commits phase-oriented and small enough to review.
- Keep desktop dependency checks separate from the UI-independent Go core.
- Keep the self-documenting `Makefile` as the local/CI entry point so development commands do not drift from GitHub Actions.

## Phase 0 — Repository foundation — COMPLETE

- [x] Create Go module and repository layout.
- [x] Add architecture and roadmap documentation.
- [x] Add a self-documenting `Makefile` with `make help` and grouped development/CI targets.
- [x] Add GitHub Actions for formatting, vetting and race-enabled tests.
- [x] Establish application metadata/version package.

Exit criteria: repository builds and CI executes the same core checks used locally.

## Phase 1 — Linux device discovery and safety — COMPLETE

- [x] Discover whole disks using structured `lsblk` JSON.
- [x] Aggregate nested partitions, dm-crypt and LVM metadata.
- [x] Detect system mounts and swap descendants.
- [x] Classify removable/external media conservatively.
- [x] Produce machine-readable raw-test allow/deny reasons.
- [x] Cover malformed metadata, system disks, mounted media, read-only media and external media in tests.

Exit criteria: discovery never writes to storage and unsafe/raw-ineligible devices are denied by default.

## Phase 2 — Non-destructive filesystem verifier — COMPLETE

- [x] Write deterministic verification data inside a mounted filesystem.
- [x] Flush data with `fsync` before verification.
- [x] Read back and detect mismatches/corruption.
- [x] Emit structured progress events.
- [x] Support context cancellation.
- [x] Remove temporary test data on success, cancellation and ordinary failure.
- [x] Add corruption-injection tests using temporary directories.

Exit criteria: filesystem verification is independently testable without USB hardware.

## Phase 3 — Wails/Svelte desktop application — COMPLETE

### 3.1 Application service contract

- [x] Add a UI-facing Go service that returns device cards without exposing mutable internals.
- [x] Include raw-test safety state and reasons in each device view model.
- [x] Add stable formatting helpers/fields needed by the frontend.
- [x] Unit-test mapping, ordering and safety propagation.

### 3.2 Wails shell

- [x] Add Wails v2.15 application entry point and project configuration.
- [x] Bind only the application service, not low-level storage packages directly.
- [x] Keep the existing CLI usable for diagnostics.
- [x] Embed production frontend assets through Wails.
- [x] Align the project with Wails v2.15's Go 1.25 minimum requirement.

### 3.3 Svelte/TypeScript frontend

- [x] Add Svelte + TypeScript frontend structure.
- [x] Build device selection screen with model/vendor, capacity, transport, mounts and safety state.
- [x] Add empty, loading and backend-error states.
- [x] Add responsive desktop layout using native keyboard-focusable controls.
- [x] Keep destructive controls absent/disabled until Phase 4.

### 3.4 Live verification UI

- [x] Define frontend region states: `pending`, `writing`, `valid`, `corrupt`, `read-error`, `write-error`.
- [x] Render a scalable region grid inspired by ValiDrive without copying its branding/assets.
- [x] Display phase, elapsed time and overall progress.
- [x] Provide a concurrency-safe Go session manager that can cancel the active verification and reject duplicate starts.
- [x] Wire the Wails/Svelte Stop action to the Go cancellation manager.
- [x] Run the existing filesystem verifier only on an explicitly selected mounted filesystem, after refreshing device identity and mount ownership.
- [x] Surface typed per-region corruption/read/write failures from the verifier into the live map with region-level failure detail.

### 3.5 Phase 3 tests and CI

- [x] Go unit tests for the application service.
- [x] Add injected corruption, read-error and write-error tests for region failure propagation.
- [x] Frontend unit tests for formatting, device-state mapping, progress calculation, typed outcomes and region-state reducer.
- [x] Frontend dependency audit, Svelte type check and production build in CI.
- [x] Race-enabled core tests, vetting, formatting and tidy-module checks in CI.
- [x] Wails v2.15 build smoke test on Ubuntu 24.04 with WebKitGTK 4.1.
- [x] Commit reproducible Go/npm dependency lock data and use `npm ci` for frontend/Wails builds.
- [x] Route local and GitHub Actions validation through self-documenting `make ci-*` targets and smoke-test `make help`.

Exit criteria: the desktop app lists real devices, explains safety state, can run/cancel the non-destructive filesystem verifier, visualizes success/failure per region, builds reproducibly from committed dependency lock data, and exposes no raw-device write capability.

## Phase 4 — Raw destructive capacity probe — PLANNED

### 4.1 Probe design

- [ ] Define the threat model: fake capacity, alias/wraparound, unreadable regions and inconsistent writes.
- [ ] Define sample layout across the advertised address space.
- [ ] Decide and document reversible sampling vs explicitly destructive modes.
- [ ] Use direct block-device I/O in an isolated package.

### 4.2 Safety gate

- [ ] Refresh discovery immediately before opening the device.
- [ ] Match device identity using path plus stable metadata where available.
- [ ] Reject mounted, system, swap, read-only, non-whole-disk and non-external devices.
- [ ] Require an explicit UI confirmation describing data-loss risk.
- [ ] Prevent stale UI selections from authorizing a changed/reinserted device.

### 4.3 Probe engine

- [ ] Read/save original sampled bytes where restoration is supported.
- [ ] Write unique patterns per sampled region.
- [ ] Flush device buffers where supported.
- [ ] Re-read in an order capable of detecting alias/wraparound.
- [ ] Classify every sample with structured result codes.
- [ ] Attempt restoration where promised and report restoration failures separately.
- [ ] Support cancellation without falsely reporting a clean/restored state.

### 4.4 Validation

- [ ] Unit-test address/sample planning without real devices.
- [ ] Add file-backed fake-block-device tests for aliasing and short-device behavior.
- [ ] Document hardware-in-the-loop test procedure using sacrificial USB media.
- [ ] Never run destructive hardware tests in ordinary CI.

Exit criteria: raw probing cannot be started against a currently unsafe device and fake-capacity behavior is detected reproducibly.

## Phase 5 — Reports, packaging and release — PLANNED

- [ ] Generate human-readable verification report.
- [ ] Generate stable machine-readable JSON report schema.
- [ ] Include device identity, advertised capacity, tested capacity, errors, timestamps and app version.
- [ ] Add AppImage packaging.
- [ ] Evaluate `.deb`, `.rpm` and Flatpak packaging.
- [ ] Add application icon and original visual identity.
- [ ] Add release workflow with tagged builds and checksums.
- [ ] Add changelog and semantic versioning policy.
- [ ] Choose and add an open-source license before the first public release.
- [ ] Perform accessibility review and destructive-operation UX review.

Exit criteria: reproducible release artifacts, understandable reports and documented installation paths.

## Backlog after first release

- [ ] Windows device-discovery backend and safety policy.
- [ ] macOS device-discovery backend if Wails support and raw-I/O semantics are satisfactory.
- [ ] Optional benchmark mode separated from integrity/capacity verification.
- [ ] Historical reports without storing sensitive device data by default.
- [ ] Localization framework.
- [ ] Automatic update strategy only after signing/release infrastructure is mature.
