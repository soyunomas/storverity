# StorVerity

StorVerity is a Linux-first storage verification tool for detecting fake-capacity, corrupted, or unreliable USB drives, SD cards, and other removable media.

> **Status:** early development. The desktop application includes both the guarded non-destructive filesystem verifier and a guarded destructive raw capacity probe. Raw probing can cause data loss and should only be used on expendable media.

## Current capabilities

- Discover whole-disk block devices on Linux through structured `lsblk` JSON.
- Aggregate mount points and filesystems from nested partitions, dm-crypt/LVM stacks, and other descendants.
- Identify disks that contain critical system mount points and swap.
- Evaluate raw-test eligibility with explicit machine-readable deny/warning reasons.
- Non-destructive filesystem verification with deterministic region data, `fsync`, read-back verification, cancellation, progress events, cleanup, and typed per-region corruption/read/write failures.
- Sampled destructive raw capacity probing with unique patterns across the advertised address space, reverse-order verification for alias/wraparound detection, structured per-sample outcomes, cancellation, and best-effort restoration of touched blocks.
- A short-lived one-time destructive confirmation challenge tied to refreshed device identity, plus pre-open and post-open safety revalidation.
- Linux raw targets opened synchronously and exclusively, with descriptor `major:minor` verification before the first write.
- Wails v2 + Svelte/TypeScript desktop UI with device selection, safety state, live region maps, progress, Stop/restore support, and explicit destructive-risk messaging.
- Reproducible Go/npm dependency metadata with committed `go.sum` and `package-lock.json`; frontend and Wails builds install with `npm ci`.
- CI coverage for the Go core, raw-probe fakes, dependency consistency, frontend tests/type checking/security audit/production build, and a native Wails build on Ubuntu 24.04 with WebKitGTK 4.1.

## Requirements

- Go 1.25 or newer (required by Wails v2.15).
- Node.js 22 for frontend development.
- Linux desktop development packages required by Wails/WebKitGTK.
- Operating-system permission to open a selected raw block device read/write when using the destructive raw probe.

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
make ci             # full CI-equivalent validation, including desktop build
```

Useful development targets include:

```bash
make list            # diagnostic JSON device discovery
make test-rawprobe   # raw engine/safety tests; never touches real block devices
make dev             # Wails desktop development mode
make build           # clean production desktop build
make frontend-dev    # standalone Vite dev server
make lock-check      # verify go.mod/go.sum/package-lock consistency
make lock-update     # intentionally regenerate dependency lock data
make test-cover-html # Go coverage report
make clean           # generated artifacts, preserving tracked placeholders
```

Run `make help` for the complete target list. CI intentionally calls the same `make ci-*` targets used locally so the workflow does not duplicate validation commands.

Core and CLI work can still be validated without the desktop toolchain with `make check-core`. Production Linux desktop builds use the `webkit2_41` Wails build tag by default; it can be overridden with `WAILS_TAGS=...` when needed.

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
| 5 | Reports, packaging, releases | Planned |

See [`todo.md`](todo.md) for the live implementation checklist, [`docs/roadmap.md`](docs/roadmap.md) for phase acceptance criteria, and [`docs/raw-probe.md`](docs/raw-probe.md) for the raw-probe threat model and safety design.

## Safety model

The raw capacity probe is **destructive even though StorVerity attempts to restore every sampled block**. Power loss, disconnects, fraudulent firmware, write failures, or restoration failures can leave data damaged. Use expendable media and keep backups of anything important.

Raw probing is blocked when a device is mounted, read-only, identified as a system disk, contains swap, is not a whole external/removable disk, has an invalid device path, or fails other safety checks. The UI never treats a manually typed `/dev/...` path as authorization.

Preparing a raw probe creates a short-lived one-use challenge tied to the selected device identity. Starting it refreshes discovery and safety before opening the target. Linux opens the block device with synchronous exclusive access and verifies the opened descriptor's `major:minor`; discovery and safety are then refreshed a second time before the first raw write. A stale/reinserted target or a filesystem mounted during that window aborts the run.

The default raw profile samples 512 locations with 4 KiB unique patterns (about 2 MiB total pattern data), verifies them in reverse write order to expose alias/wraparound behavior, and restores all touched blocks best-effort. A clean sampled result is not an exhaustive proof that every byte of the medium is healthy.

Ordinary CI never performs raw writes against `/dev/*`. See [`docs/raw-probe-hardware-test.md`](docs/raw-probe-hardware-test.md) for the sacrificial-media hardware test procedure.

## License

No license has been selected yet. Until one is added, normal copyright rules apply.
