# Linux packaging

StorVerity is Linux-first and currently builds release artifacts for x86_64.

## Supported release artifacts

### AppImage

`make package-appimage VERSION=x.y.z` builds `dist/StorVerity-x.y.z-x86_64.AppImage` from the Wails production binary.

The packaging script downloads the official AppImage `appimagetool` and type-2 runtime from their upstream release repositories and verifies pinned SHA-256 digests before use. The AppImage contains StorVerity, its desktop entry, icon and AppStream metadata.

The current Wails/Linux binary dynamically uses system GTK/WebKitGTK libraries. The AppImage therefore provides a convenient single application artifact but does not claim to bundle a complete desktop stack. Distributions still need compatible GTK 3 and WebKitGTK 4.1 runtime libraries.

### Deterministic tarball

`make package-tarball VERSION=x.y.z SOURCE_DATE_EPOCH=<epoch>` builds `dist/StorVerity-x.y.z-linux-x86_64.tar.gz`. File ordering, timestamps and ownership metadata are normalized so the archive is deterministic for the same binary and source epoch.

`make package` builds both formats and creates `dist/SHA256SUMS`. `make ci-package` performs the same packaging/checksum validation against an existing CI desktop binary.

## Debian / Ubuntu (`.deb`) evaluation

A native `.deb` is technically straightforward and can be produced from the existing desktop/AppStream metadata with tools such as `dpkg-deb` or a packaging generator. It should declare runtime dependencies appropriate to the target distribution, notably GTK 3 and WebKitGTK 4.1.

It is deferred until StorVerity has a tested distro support matrix. WebKitGTK package names and minimum versions vary between supported Debian/Ubuntu generations, so publishing one nominal `.deb` without validating those dependencies would be misleading.

## Fedora / RPM (`.rpm`) evaluation

An RPM can use the same installed file layout and metadata. As with Debian packages, the dependency names and WebKitGTK ABI availability need to be verified against the Fedora/RHEL-family versions StorVerity intends to support. RPM packaging is therefore feasible but intentionally deferred until that compatibility matrix exists.

## Flatpak evaluation

Flatpak is attractive for the non-destructive verifier, but StorVerity's raw capacity probe intentionally requires tightly controlled access to host block devices. Broad device permissions would weaken the sandbox, while a privileged host helper would require an additional authenticated IPC/polkit design.

For that reason Flatpak is deferred until raw I/O is split into a minimal privileged helper. StorVerity will not request blanket device access merely to make sandbox packaging appear complete.

## Installation and permissions

Filesystem verification works with normal user access to the selected mounted filesystem. Raw probing additionally requires operating-system permission to open the whole block device read/write and exclusively. Packaging does not make the entire GUI run as root and does not install a permissive udev rule automatically.

A future privileged-helper/polkit design may provide narrowly scoped elevation. Until then users who need raw probing must deliberately launch/configure StorVerity with appropriate block-device permissions and should use only expendable media.
