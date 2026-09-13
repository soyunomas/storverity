# StorVerity

StorVerity is a Linux-first storage verification tool for detecting fake-capacity, corrupted, or unreliable USB drives, SD cards, and other removable media.

> **Status:** early development. The current code is read-only and does not write to block devices.

## Current capabilities

- Discover whole-disk block devices on Linux through structured `lsblk` JSON.
- Aggregate mount points and filesystems from nested partitions, dm-crypt/LVM stacks, and other descendants.
- Identify disks that contain critical system mount points.
- Flag disks containing swap.
- Evaluate whether a disk is eligible for a future raw destructive test, with explicit deny/warning reasons.
- Expose discovery and safety information through a temporary JSON CLI.

```bash
go test ./...
go run ./cmd/storverity list
```

## Planned desktop stack

The desktop application will use **Go + Wails v2 + Svelte/TypeScript**. Core storage logic stays independent from Wails so it can be tested without a GUI or real removable media.

## Development phases

| Phase | Scope | Status |
| --- | --- | --- |
| 0 | Repository foundation, architecture, CI | Complete |
| 1 | Linux device discovery and safety policy | Complete |
| 2 | Non-destructive filesystem verification engine | Planned |
| 3 | Wails/Svelte desktop UI and live progress map | Planned |
| 4 | Raw destructive capacity probe | Planned |
| 5 | Reports, packaging, releases | Planned |

See [`docs/roadmap.md`](docs/roadmap.md) for acceptance criteria.

## Safety model

StorVerity is intended to perform destructive tests eventually. Raw testing will be blocked when a device is mounted, read-only, identified as a system disk, contains swap, has an invalid device path, or fails other safety checks. The UI will never rely on a manually typed device path as sufficient authorization for destructive I/O.

## License

No license has been selected yet. Until one is added, normal copyright rules apply.
