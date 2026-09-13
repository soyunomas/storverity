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
3. Create and push a tag named `vMAJOR.MINOR.PATCH` (or a valid prerelease tag).
4. The Release workflow validates the tag/changelog, runs the full validation suite, builds the Linux release artifacts and writes SHA-256 checksums.
5. GitHub Actions obtains a short-lived OIDC identity and cosign signs every release blob keylessly, producing a `.sigstore.json` verification bundle per artifact.
6. The workflow immediately verifies each bundle against the exact repository/workflow/tag identity before publishing the GitHub Release.

The release workflow does not modify source files. Release metadata is injected into the binary with linker flags, keeping tagged source trees reproducible.

## Signing policy

StorVerity does not keep a long-lived private release key in the repository or GitHub secrets. Release artifacts use Sigstore/cosign keyless signing backed by the GitHub Actions OIDC identity. Each artifact and `SHA256SUMS` receives a verification bundle that is published alongside the release files.

A verifier should validate both the artifact digest/signature bundle and the certificate identity for `soyunomas/storverity/.github/workflows/release.yml` at the expected tag. This ties the signature to the repository release workflow rather than to an exportable private key.

Checksums remain useful for simple integrity checking and mirrors, while Sigstore bundles provide cryptographic provenance for the official tagged build.
