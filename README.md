# StorVerity

StorVerity is a Linux-first storage verification tool for detecting fake-capacity, corrupted, or unreliable USB drives, SD cards, and other removable media.

> **Status:** early development. Raw block-device writes remain disabled. The desktop application currently exposes only the guarded non-destructive filesystem verifier.

## Current capabilities

- Discover whole-disk block devices on Linux through structured `lsblk` JSON.
- Aggregate mount points and filesystems from nested partitions, dm-crypt/LVM stacks, and other descendants.
- Identify disks that contain critical system mount points and swap.
- Evaluate future raw-test eligibility with explicit machine-readable deny/warning reasons.
- Non-destructive filesystem verification with deterministic region data, `fsync`, read-back verification, cancellation, progress events, cleanup, and typed per-region corruption/read/write failures.
- Wails v2 + Svelte/TypeScript desktop UI with device selection, safety state, mount/test-size controls, live region grid, progress and Stop support.
- Reproducible Go/npm dependency metadata with committed `go.sum` and `package-lock.json`; frontend and Wails builds install with `npm ci`.
- CI coverage for the Go core, dependency consistency, frontend tests/type checking/security audit/production build, and a native Wails build on Ubuntu 24.04 with WebKitGTK 4.1.

## Requirements

- Go 1.25 or newer (required by Wails v2.15).
- Node.js 22 for frontend development.
- Linux desktop development packages required by Wails/WebKitGTK.

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
| 4 | Raw destructive capacity probe | Planned |
| 5 | Reports, packaging, releases | Planned |

See [`todo.md`](todo.md) for the live implementation checklist and [`docs/roadmap.md`](docs/roadmap.md) for phase acceptance criteria.

## Safety model

StorVerity is intended to perform destructive tests eventually. Raw testing will be blocked when a device is mounted, read-only, identified as a system disk, contains swap, has an invalid device path, or fails other safety checks. The UI never treats a manually typed `/dev/...` path as sufficient authorization for destructive I/O.

The current desktop verifier operates only inside a selected mounted filesystem. Immediately before starting, StorVerity refreshes device discovery and verifies that the selected mount still belongs to the selected external device.

## License

No license has been selected yet. Until one is added, normal copyright rules apply.
