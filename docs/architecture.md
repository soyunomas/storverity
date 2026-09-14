# Architecture

StorVerity is split into a testable Go core, an unprivileged desktop presentation layer, and a minimal privileged Linux helper used only for destructive raw probing.

```text
Wails + Svelte UI (unprivileged)
       |
       v
Application service / bindings
       |
       +-- device discovery
       +-- safety policy
       +-- filesystem verification engine
       +-- report generator
       +-- progress/event stream
       |
       +-- raw-probe confirmation
               |
               v
        system D-Bus client
               |
          polkit action
               |
               v
storverity-helper (root, D-Bus activated)
       |
       +-- fresh device discovery + safety policy
       +-- fingerprint / major:minor validation
       +-- exclusive block-device open
       +-- second discovery/safety validation
       +-- raw capacity probe engine
```

## Design rules

1. **Discovery is read-only.** Enumerating devices must never open a block device for writing.
2. **Core logic is UI-independent.** Storage algorithms live in normal Go packages.
3. **The desktop process stays unprivileged.** Wails never opens `/dev/...` read/write for the raw probe.
4. **Destructive I/O is isolated.** Only the root helper can open a raw block device, and it exposes a high-level probe operation rather than a generic read/write API.
5. **System storage is denied by default.** A whole disk is treated as a system disk if any descendant contains `/`, `/boot`, `/boot/efi`, `/home`, `/usr`, or `/var`.
6. **Mounted media is never raw-tested.** All descendant mount points must be absent before destructive access.
7. **The UI cannot turn a free-form `/dev/...` string directly into a destructive operation.** It selects from fresh backend discovery results and passes a stable device ID plus confirmed fingerprint to the helper.
8. **The helper is authoritative.** It repeats discovery, safety, identity and `major:minor` checks before and after opening the target, immediately before the first write.
9. **Verification results are structured.** The visual region map consumes per-region result objects, not parsed human-readable command output.
10. **Cancellation crosses the privilege boundary.** Raw-probe sessions are scoped to the D-Bus caller/session and are cancelled when requested or when the caller disconnects.

## Linux discovery

The Linux backend uses `lsblk --json --bytes --paths`. `lsblk` is used only for metadata discovery; test I/O is implemented in Go rather than by parsing F3 output.

A disk record aggregates metadata recursively from all descendants. This is important for layouts such as:

```text
/dev/nvme0n1
└─/dev/nvme0n1p3
  └─/dev/mapper/cryptroot
    └─/dev/mapper/vg-root  /
```

The physical disk must still be classified as system storage even though `/` is mounted several layers below it.

## Privileged raw-probe boundary

The desktop first creates the existing short-lived, one-use destructive confirmation challenge. After exact confirmation it revalidates the selected device locally and sends the system helper only a generated session ID, stable device ID, expected fingerprint, and bounded probe geometry.

The helper obtains polkit authorization for `io.github.soyunomas.StorVerity.raw-probe`, rediscovers the target by stable ID, repeats the raw-test safety policy, verifies the fingerprint, opens the discovered path exclusively while checking `major:minor`, and then rediscovers/revalidates once more before calling the raw-probe engine. A stale/reinserted device or a mount appearing during that window aborts the operation.

See [`privileged-helper.md`](privileged-helper.md) for the D-Bus/polkit contract, installation model, and manual integration checks.
