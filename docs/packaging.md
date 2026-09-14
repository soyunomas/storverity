# Linux packaging

StorVerity is Linux-first and currently builds release artifacts for x86_64.

## Supported release artifacts

### AppImage

`make package-appimage VERSION=x.y.z` builds `dist/StorVerity-x.y.z-x86_64.AppImage` from the Wails production binary.

The packaging script downloads the official AppImage `appimagetool` and type-2 runtime from their upstream release repositories and verifies pinned SHA-256 digests before use. The AppImage contains StorVerity, its desktop entry, icon and AppStream metadata.

The current Wails/Linux binary dynamically uses system GTK/WebKitGTK libraries. The AppImage therefore provides a convenient single application artifact but does not claim to bundle a complete desktop stack. Distributions still need compatible GTK 3 and WebKitGTK 4.1 runtime libraries.

The AppImage remains unprivileged and deliberately does not install root-owned D-Bus, polkit, or systemd files. Filesystem verification works directly from the AppImage. Destructive raw probing requires the matching `storverity-helper` to be installed separately on the host.

### Deterministic tarball

`make package-tarball VERSION=x.y.z SOURCE_DATE_EPOCH=<epoch>` builds `dist/StorVerity-x.y.z-linux-x86_64.tar.gz`. File ordering, timestamps and ownership metadata are normalized so the archive is deterministic for the same binaries and source epoch.

The tarball contains the desktop binary plus the privileged helper and its installation assets. After extraction, install the privileged component with:

```bash
sudo ./install-helper.sh
```

The installer places the helper under `/usr/libexec`, installs the polkit action, system D-Bus activation/ownership policy, and the hardened systemd unit. The GUI itself remains a normal-user process.

`make package` builds both formats and creates `dist/SHA256SUMS`. `make ci-package` performs the same packaging/checksum validation against an existing CI desktop binary and builds the helper without performing privileged installation or raw I/O.

See [`privileged-helper.md`](privileged-helper.md) for the privilege-separation security model and manual integration procedure.

## Debian / Ubuntu (`.deb`) evaluation

A native `.deb` is technically straightforward and can be produced from the existing desktop/AppStream metadata plus the helper installation layout. It should declare runtime dependencies appropriate to the target distribution, notably GTK 3, WebKitGTK 4.1, D-Bus, polkit, and systemd integration where used.

It remains deferred until StorVerity has a tested distro support matrix. WebKitGTK package names and minimum versions vary between supported Debian/Ubuntu generations, so publishing one nominal `.deb` without validating those dependencies would be misleading.

## Fedora / RPM (`.rpm`) evaluation

An RPM can use the same installed file layout and privilege-separation model. As with Debian packages, dependency names and WebKitGTK ABI availability need to be verified against the Fedora/RHEL-family versions StorVerity intends to support. RPM packaging is therefore feasible but intentionally deferred until that compatibility matrix exists.

## Flatpak evaluation

The privileged helper removes the need to grant the Wails process blanket host block-device access, which is the right architectural prerequisite for sandbox packaging. A production Flatpak still needs a carefully reviewed host-helper installation/discovery story and portal/sandbox compatibility testing before it can be claimed as supported.

Flatpak therefore remains deferred, but no longer because the application lacks privilege separation. StorVerity will not request broad device permissions merely to make sandbox packaging appear complete.

## Installation and permissions

Filesystem verification works with normal user access to the selected mounted filesystem. Raw probing is requested by the unprivileged desktop over the system D-Bus. The root `storverity-helper` performs per-operation polkit authorization, repeats discovery/safety/identity checks, and is the only component that opens the whole block device read/write and exclusively.

Packaging does not run the Wails/WebKit process as root and does not install a permissive udev rule. The helper exposes a narrow raw-probe operation rather than general block-device access.
