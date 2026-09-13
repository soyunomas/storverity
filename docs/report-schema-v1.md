# StorVerity report schema v1

StorVerity can export each completed, failed or cancelled storage operation as human-readable text or machine-readable JSON. The JSON contract is identified by `schemaVersion: "storverity.report.v1"` and its machine-readable JSON Schema is committed at `internal/report/schema-v1.json`.

## Common envelope

Every report records:

- application name, version, source commit and build date when available;
- operation type (`filesystem` or `raw`) and final status;
- UTC start/completion timestamps and duration in milliseconds;
- selected device identity, path, model/vendor/serial/transport where available, and advertised capacity;
- the number of bytes actually tested by that operation;
- a non-null `errors` array containing structured error codes and optional region/sample/offset context.

The report deliberately keeps device identity because it is needed to associate a result with the tested medium. Serial numbers can be identifying data; users should review exported reports before publishing them publicly.

## Filesystem reports

The `filesystem` object records the selected mount point, requested byte count, successfully written/verified byte counts and number of verification regions.

`testedCapacityBytes` is the number of bytes successfully read back and verified, not the total size of the filesystem or device.

## Raw reports

The `raw` object records advertised capacity, probe block size, sample counts, I/O and restoration error counts, the highest consecutively validated address, fake-capacity suspicion, restoration state and every sampled result.

`testedCapacityBytes` is the total sampled payload (`samples × blockBytes`). A raw sampled report is a fraud-detection result, not proof that every byte of the device is healthy.

## Status values

- `passed`: the requested verification completed without a reported integrity or restoration failure.
- `suspicious`: raw probing completed but indicates fake-capacity behavior.
- `failed`: execution or restoration failed.
- `cancelled`: the operation was cancelled; raw probing still attempts restoration of touched samples before returning.

## Compatibility

Consumers should inspect `schemaVersion` before parsing operation-specific fields. Additive optional fields may appear in v1, but incompatible changes require a new schema identifier such as `storverity.report.v2` and a separate schema file.
