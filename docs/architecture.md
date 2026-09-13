# Architecture

StorVerity is split into a testable Go core and a desktop presentation layer.

```text
Wails + Svelte UI
       |
       v
Application service / bindings
       |
       +-- device discovery
       +-- safety policy
       +-- filesystem verification engine
       +-- raw capacity probe
       +-- progress/event stream
       +-- report generator
```

## Design rules

1. **Discovery is read-only.** Enumerating devices must never open a block device for writing.
2. **Core logic is UI-independent.** Storage algorithms live in normal Go packages.
3. **Destructive I/O is isolated.** The raw probe gets its own package and must pass a safety decision immediately before opening the device.
4. **System storage is denied by default.** A whole disk is treated as a system disk if any descendant contains `/`, `/boot`, `/boot/efi`, `/home`, `/usr`, or `/var`.
5. **Mounted media is never raw-tested.** All descendant mount points must be absent before destructive access.
6. **The UI cannot turn a free-form `/dev/...` string directly into a destructive operation.** It selects from fresh backend discovery results.
7. **Verification results are structured.** The visual region map consumes per-region result objects, not parsed human-readable command output.

## Linux discovery

The first backend implementation uses `lsblk --json --bytes --paths`. `lsblk` is used only for metadata discovery; test I/O will be implemented in Go rather than by parsing F3 output.

A disk record aggregates metadata recursively from all descendants. This is important for layouts such as:

```text
/dev/nvme0n1
└─/dev/nvme0n1p3
  └─/dev/mapper/cryptroot
    └─/dev/mapper/vg-root  /
```

The physical disk must still be classified as system storage even though `/` is mounted several layers below it.
