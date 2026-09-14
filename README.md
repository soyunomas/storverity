# StorVerity

StorVerity is a Linux-first storage verification tool for detecting fake-capacity, corrupted, or unreliable USB drives, SD cards, and other removable media.

> **Status:** active early release development. The desktop application includes a guarded non-destructive filesystem verifier and a guarded destructive raw capacity probe. Raw probing can cause data loss and should only be used on expendable media.

## Current capabilities

- Discover whole-disk block devices on Linux through structured `lsblk` JSON.
- Aggregate mount points and filesystems from nested partitions, dm-crypt/LVM stacks, and other descendants.
- Identify disks that contain critical system mount points and swap.
- Evaluate raw-test eligibility with explicit machine-readable deny/warning reasons.
- Non-destructive filesystem verification with deterministic region data, `fsync`, read-back verification, cancellation, progress events, cleanup, and typed per-region corruption/read/write failures.
- Sampled destructive raw capacity probing with unique patterns across the advertised address space, reverse-order verification for alias/wraparound detection, structured per-sample outcomes, cancellation, and best-effort restoration of touched blocks.
- A short-lived one-time destructive confirmation challenge tied to refreshed device identity, plus pre-open and post-open safety revalidation.
- Privilege-separated raw probing: the Wails/WebKit desktop stays unprivileged and delegates destructive block-device access to a minimal system D-Bus helper authorized per operation with polkit.
- The root helper independently rediscovers the target, repeats the raw safety policy, checks a confirmed device fingerprint, opens the block device synchronously/exclusively, verifies descriptor `major:minor`, and revalidates again before the first write.
- Session-scoped raw-probe cancellation across D-Bus, including cancellation when the desktop caller disconnects.
- Wails v2 + Svelte/TypeScript desktop UI with device selection, safety state, live region maps, progress, Stop/restore support, report export, keyboard focus treatment, reduced-motion support, and explicit destructive-risk messaging.
- Human-readable TXT reports and stable `storverity.report.v1` JSON reports containing app/build metadata, device identity, tested/advertised capacity, timestamps, status and structured errors.
- x86_64 AppImage and deterministic Linux tarball packaging with SHA-256 manifests; the tarball also contains the privileged-helper installation assets.
- Tag-driven GitHub releases with SemVer/changelog validation, embedded build metadata, keyless Sigstore signing bundles, and release checksums.
- Reproducible Go/npm dependency metadata with committed `go.sum` and `package-lock.json`; frontend and Wails builds install with `npm ci`.
- CI coverage for the Go core, privileged-helper boundary, raw-probe fakes, report/schema tests, dependency consistency, frontend tests/type checking/security audit/production build, native Wails build, AppImage/tarball packaging and checksum verification on Ubuntu 24.04.

## Requirements

- Go 1.25 or newer (required by Wails v2.15).
- Node.js 22 for frontend development.
- Linux desktop development packages required by Wails/WebKitGTK.
- System D-Bus and polkit for privilege-separated destructive raw probing.
- The matching `storverity-helper` installed on the host when using the destructive raw probe.

## Development

The project uses a self-documenting `Makefile` as the main development interface. Start with:

```bash
make help
make doctor
```

Typical setup and validation:

```bash
make linux-deps     # Ubuntu/Debian desktop packages
make bootstrap      # locked Go/frontend dependencies + pinned Wails CLI
make check          # Go core + frontend validation
make ci             # full CI-equivalent validation, including desktop/package smoke tests
```

Useful development targets include:

```bash
make list            # diagnostic JSON device discovery
make test-rawprobe   # raw/helper/safety tests; never touches real block devices
make helper-build    # build the privileged Linux helper
make helper-install  # install helper integration locally (sudo; Linux only)
make dev             # Wails desktop development mode
make build           # clean production desktop build
make package         # AppImage + deterministic tarball + SHA256SUMS
make frontend-dev    # standalone Vite dev server
make lock-check      # verify go.mod/go.sum/package-lock consistency
make lock-update     # intentionally regenerate dependency lock data
make test-cover-html # Go coverage report
make clean           # generated artifacts, preserving tracked placeholders
```

Run `make help` for the complete target list. CI intentionally calls the same `make ci-*` targets used locally so the workflow does not duplicate validation commands.

Core and CLI work can still be validated without the desktop toolchain with `make check-core`. Production Linux desktop builds use the `webkit2_41` Wails build tag by default; it can be overridden with `WAILS_TAGS=...` when needed.

## Reports

After a filesystem verification or raw capacity probe, the desktop UI can save the latest result as TXT or JSON. JSON exports use the stable `storverity.report.v1` envelope and are validated against the committed schema in `internal/report/schema-v1.json`.

Reports include device identity data (including serial number when exposed by the OS). Treat exported reports as potentially identifying before publishing them. See [`docs/report-schema-v1.md`](docs/report-schema-v1.md) for the compatibility contract.

## Packaging and releases

`make package VERSION=x.y.z` builds an x86_64 AppImage, a deterministic Linux tarball and `SHA256SUMS`. The tarball includes `storverity-helper`, its polkit/D-Bus/systemd files, and `install-helper.sh`. From an extracted tarball, install the helper with `sudo ./install-helper.sh` before using raw probing.

The AppImage deliberately stays self-contained only for the unprivileged GUI and does not self-install root files. Filesystem verification works from the AppImage directly; raw probing requires a matching helper already installed on the host.

The release workflow runs from SemVer-style `vMAJOR.MINOR.PATCH` tags, requires a matching changelog entry, embeds version/commit/build date, signs every release artifact keylessly with Sigstore/cosign, verifies the generated bundles, and publishes the artifacts plus signature bundles to GitHub Releases.

Native `.deb`/RPM packaging is deferred until a tested distro support matrix exists. Flatpak now has the required privilege-separation architecture but remains deferred until host-helper/sandbox integration is validated. See [`docs/packaging.md`](docs/packaging.md), [`docs/privileged-helper.md`](docs/privileged-helper.md), and [`docs/versioning.md`](docs/versioning.md).

## Desktop stack

The desktop application uses **Go 1.25 + Wails v2.15 + Svelte 5/TypeScript**. Storage algorithms remain independent from Wails so they can be race-tested without a GUI or real removable media. The UI binds only the application service rather than low-level device or storage packages.

## Development phases

| Phase | Scope | Status |
| --- | --- | --- |
| 0 | Repository foundation, architecture, CI | Complete |
| 1 | Linux device discovery and safety policy | Complete |
| 2 | Non-destructive filesystem verification engine | Complete |
| 3 | Wails/Svelte desktop UI and live progress map | Complete |
| 4 | Raw destructive capacity probe | Complete |
| 5 | Reports, packaging, releases | Complete |
| 6 | Privilege-separated raw I/O with polkit/D-Bus helper | Complete |

See [`todo.md`](todo.md) for the implementation checklist, [`docs/roadmap.md`](docs/roadmap.md) for the original phase acceptance criteria, [`docs/raw-probe.md`](docs/raw-probe.md) for the raw-probe threat model, [`docs/privileged-helper.md`](docs/privileged-helper.md) for the privilege boundary, and [`docs/accessibility-and-destructive-ux.md`](docs/accessibility-and-destructive-ux.md) for the Phase 5 UX review.

## Safety model

The raw capacity probe is **destructive even though StorVerity attempts to restore every sampled block**. Power loss, disconnects, fraudulent firmware, write failures, or restoration failures can leave data damaged. Use expendable media and keep backups of anything important.

Raw probing is blocked when a device is mounted, read-only, identified as a system disk, contains swap, is not a whole external/removable disk, has an invalid device path, or fails other safety checks. The UI never treats a manually typed `/dev/...` path as authorization.

Preparing a raw probe creates a short-lived one-use challenge tied to the selected device identity. Starting it refreshes discovery and safety in the unprivileged application service, then sends only a stable device ID, expected fingerprint, session ID, and bounded probe geometry over the system D-Bus. The root helper obtains polkit authorization and independently repeats discovery/safety/identity checks before opening the target. Linux opens the block device with synchronous exclusive access and verifies the opened descriptor's `major:minor`; the helper refreshes discovery and safety a second time before the first raw write. A stale/reinserted target or a filesystem mounted during that window aborts the run.

The default raw profile samples 512 locations with 4 KiB unique patterns (about 2 MiB total pattern data), verifies them in reverse write order to expose alias/wraparound behavior, and restores all touched blocks best-effort. A clean sampled result is not an exhaustive proof that every byte of the medium is healthy.

Ordinary CI never performs raw writes against `/dev/*`. See [`docs/raw-probe-hardware-test.md`](docs/raw-probe-hardware-test.md) for the sacrificial-media hardware test procedure and [`docs/privileged-helper.md`](docs/privileged-helper.md) for the polkit/D-Bus integration checks.

## License

StorVerity is released under the MIT License. See [`LICENSE`](LICENSE).
