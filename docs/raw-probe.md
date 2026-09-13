# Raw capacity probe design

Phase 4 introduces direct block-device writes. The probe is intentionally isolated from Wails and device discovery so the storage algorithm can be tested with ordinary in-memory/file-backed media.

## Threat model

The probe is designed to detect:

- firmware that advertises more address space than physically exists;
- address aliasing/wraparound where writes to high logical offsets overwrite lower storage;
- regions that accept writes but do not retain the expected data;
- short/unreadable regions and explicit write failures;
- stale or reinserted devices between UI selection and opening the device.

It does not prove that every byte of a drive is healthy. Sampling can miss localized faults. An exhaustive surface scan is a different mode and is intentionally not implied by a clean raw-probe result.

## Sampling profile

The default profile uses **512 aligned samples of 4096 bytes**, approximately 2 MiB of unique verification patterns distributed across the advertised address space. The application layer enforces at least this sample count so a manipulated frontend request cannot silently weaken the probe. The core supports up to 4096 samples and accepts power-of-two block sizes from 512 bytes through 1 MiB.

The larger default profile is intentional: it puts more unique data behind the controller than a very small spot check while still completing quickly compared with a full-media write/read test.

## Write model

StorVerity v0.x uses **sampled raw writes with best-effort restoration**. Before writing a sample, the original block is read into memory. Unique deterministic patterns are then written at aligned offsets distributed across the advertised capacity, flushed, and read back in reverse order. Reading in a different order from writing makes alias/wraparound fraud visible because later writes can overwrite earlier logical addresses.

After verification, every touched block is rewritten from its saved snapshot and the device is flushed again. Restoration is attempted after normal completion, cancellation, mismatch, and ordinary I/O failures.

This remains a destructive operation. Restoration cannot be guaranteed when power is lost, the device disconnects, firmware lies about address mapping, or restoration itself fails. The UI therefore describes possible data loss and requires an explicit per-run confirmation.

## Edge guards

When capacity permits, the planner avoids the first and last 4 MiB. This reduces the probability that an interrupted probe damages common partition-table/filesystem metadata. It is risk reduction only and must never be presented as making the raw probe safe or non-destructive.

## Safety and identity requirements

The application layer never accepts a free-form `/dev/...` value as authorization. A raw run uses a short-lived one-time challenge bound to the selected device fingerprint and exact destructive confirmation text.

Immediately before opening the target, StorVerity refreshes discovery and requires all of the following to remain true:

- the target is a whole disk;
- it is writable;
- neither it nor descendants are mounted;
- it is not a system disk and contains no swap;
- it is identified as removable/external;
- its stable selection identity still matches the prepared challenge;
- its discovered Linux `major:minor` identity is present.

The Linux opener then opens the server-selected block path with synchronous **exclusive** read/write access (`O_RDWR | O_SYNC | O_EXCL`), verifies that the opened descriptor is a block device, and compares the descriptor's real `major:minor` number with the freshly discovered value.

After the descriptor is open, StorVerity performs a **second discovery and safety revalidation before the first raw write**. A changed/reinserted device, altered identity, newly mounted filesystem, swap/system classification, or other newly unsafe state aborts the run before the probe engine receives the descriptor.

The process must also have operating-system permission to open the target read/write. A permission failure is a hard failure; StorVerity does not weaken device permissions or bypass the safety gate.

## Cancellation

Cancellation stops new probe writes as soon as the engine observes the context signal, but restoration of already touched samples deliberately ignores cancellation. The run must not report a clean/restored result until restoration has completed successfully. The desktop also prevents filesystem verification and raw probing from running concurrently within the application.

## CI policy

Ordinary CI never writes to real block devices. Planner, geometry, aliasing, short-device, restoration, cancellation, session exclusivity, safety-gate, and stale-identity behavior are tested with in-memory or file-backed fakes. A focused safe test entry point is available as:

```bash
make test-rawprobe
```

Hardware-in-the-loop testing uses sacrificial removable media and is documented in [`raw-probe-hardware-test.md`](raw-probe-hardware-test.md). No ordinary CI job invokes the raw probe against `/dev/*`.
