# Versioning and release policy

StorVerity follows Semantic Versioning 2.0.0 for application releases.

## Application versions

Release tags use `vMAJOR.MINOR.PATCH`, optionally followed by a SemVer prerelease suffix such as `v0.2.0-rc.1`. The application embeds the release version, source commit and UTC build date at compile time through Go linker variables.

While the project is below `1.0.0`, minor releases may contain significant product changes, but patch releases must remain compatible bug/security fixes within the documented interfaces of that minor line.

## Report schema versions

Machine-readable report compatibility is versioned independently inside each document. The first contract is `storverity.report.v1` and is described by `internal/report/schema-v1.json`.

Within schema v1, existing required fields and their meaning must not be removed or changed incompatibly. New optional fields may be introduced. Any incompatible report format change requires a new schema identifier and a new schema file; old schemas remain in the repository for consumers.

## Release procedure

1. Update `CHANGELOG.md` with a section matching the intended version.
2. Ensure `main` is green and the version has no unresolved release blockers.
3. Create and push an annotated or lightweight tag named `vMAJOR.MINOR.PATCH` (or a valid prerelease tag).
4. The Release workflow validates the tag and changelog, runs the full validation suite, builds the Linux release artifacts, writes SHA-256 checksums and publishes the GitHub Release.

The release workflow does not modify source files. Release metadata is injected into the binary with linker flags, keeping tagged source trees reproducible.

## Signing policy

Phase 5 publishes immutable checksums for every release artifact. Cryptographic artifact signing is intentionally not represented as complete until a long-lived signing identity and key-handling process are established; adding Sigstore or another signing mechanism is a release-hardening follow-up rather than silently storing a private signing key in the repository.
