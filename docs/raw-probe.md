# Raw capacity probe design

Phase 4 introduces direct block-device writes. The probe is intentionally isolated from Wails and device discovery so the storage algorithm can be tested with ordinary in-memory/file-backed media.

## Threat model

The probe is designed to detect:

- firmware that advertises more address space than physically exists;
- address aliasing/wraparound where writes to high logical offsets overwrite lower storage;
- regions that accept writes but do not retain the expected data;
- short/unreadable regions and explicit write failures;
- stale or reinserted devices between UI selection and opening the device.

It does not prove that every byte of a drive is healthy. Sampling can miss localized faults. A later exhaustive mode may be added separately.

## Write model

StorVerity v0.x uses **sampled raw writes with best-effort restoration**. Before writing a sample, the original block is read into memory. Unique deterministic patterns are then written at aligned offsets distributed across the advertised capacity, flushed, and read back in reverse order. Reading in a different order from writing makes alias/wraparound fraud visible because later writes can overwrite earlier logical addresses.

After verification, every touched block is rewritten from its saved snapshot and the device is flushed again. Restoration is attempted after normal completion, cancellation, mismatch, and ordinary I/O failures.

This remains a destructive operation. Restoration cannot be guaranteed when power is lost, the device disconnects, firmware lies about address mapping, or restoration itself fails. The UI must therefore describe possible data loss and require an explicit per-run confirmation.

## Edge guards

When capacity permits, the planner avoids the first and last 4 MiB. This reduces the probability that an interrupted probe damages common partition-table/filesystem metadata. It is risk reduction only and must never be presented as making the raw probe safe or non-destructive.

## Safety and identity requirements

The application layer must refresh discovery immediately before opening a raw target and reject the run unless all of the following are still true:

- the target is a whole disk;
- it is writable;
- neither it nor descendants are mounted;
- it is not a system disk and contains no swap;
- it is identified as removable/external;
- its stable selection identity still matches the prepared challenge;
- the opened Linux block device has the same kernel `major:minor` number discovered before open;
- the one-time confirmation challenge is valid and unexpired.

No free-form `/dev/...` value supplied by the frontend grants authorization.

## Cancellation

Cancellation stops new probe writes as soon as the engine observes the context signal, but restoration of already touched samples deliberately ignores cancellation. The run must not report a clean/restored result until restoration has completed successfully.

## CI policy

Ordinary CI never writes to real block devices. Planner, aliasing, short-device, restoration, cancellation, safety-gate, and stale-identity behavior are tested with in-memory or file-backed fakes. Hardware-in-the-loop tests use sacrificial removable media and are documented separately.
