# Raw probe hardware-in-the-loop procedure

This procedure is deliberately excluded from ordinary CI. It writes directly to removable media and must only be performed with **sacrificial storage whose contents may be lost**.

## Preconditions

- Use a dedicated Linux test machine or a machine with no irreplaceable removable storage attached.
- Use a USB flash drive, SD card/reader, or other external block device that can be destroyed without consequence.
- Back up anything you care about before connecting the test device.
- Run the current StorVerity desktop build from a clean checkout that has passed `make ci`.
- Keep the machine on stable power. Do not disconnect the device during a run unless deliberately testing failure handling.

## Safe identification

1. Connect exactly one sacrificial target if practical.
2. Run `make list` and record the target's model, serial, capacity, transport, path, and safety decision.
3. Confirm that the target is external/removable and is **not** the system disk.
4. Unmount every filesystem on the target. Do not merely close a file manager window.
5. Run `make list` again and verify that `mountPoints` is empty and `rawTest.allowed` is true.
6. If StorVerity reports any deny reason, stop. Do not bypass the safety gate manually.

## Desktop raw probe

1. Start the application with `make dev`.
2. Select the sacrificial device and verify the displayed model/capacity/path against the identification recorded above.
3. Choose **Prepare raw probe**.
4. Read the destructive-data warning and type the exact one-time confirmation phrase shown by the application.
5. Start the probe and observe the sample map through snapshot, write, verify, and restore activity.
6. Record the final counts for valid, corrupt, read-error, write-error, and restore-error samples.

A known-good device should normally finish with all sampled locations valid and no restoration errors. A deliberately fake/aliased test device should produce mismatches or I/O errors rather than a clean result.

## Cancellation test

Repeat the procedure and press **Stop and restore** after writes have begun. StorVerity should stop scheduling new writes and continue restoration of every sample it already touched. The run must not claim successful restoration until that restoration finishes.

## Disconnect/failure test

Only with sacrificial hardware, repeat the run and disconnect the target after writes have begun. The operation should fail. Because restoration may be impossible after disconnect, the device contents must be considered untrusted afterward. Reformat or discard the medium before reuse.

## Post-run inspection

After a normal successful run:

1. Refresh device discovery and confirm the target still has the expected identity and advertised capacity.
2. If the device previously contained a disposable filesystem, mount it and perform a filesystem check appropriate to that filesystem.
3. Do not treat a sampled clean result as proof that every byte of the medium is healthy; the raw probe is a fraud/capacity sampler, not an exhaustive surface scan.

## CI rule

Never attach a real block device to the normal GitHub Actions workflow and never add a CI step that invokes the raw probe against `/dev/*`. Automated coverage uses memory/file-backed fake devices for aliasing, short-device, corruption, cancellation, and restoration behavior.
