# Install Spare 0.1 Preview

Spare 0.1 Preview (`0.1.1-alpha.3`) is unsigned. Install it only on a test
account or a computer where you can safely remove it.

Spare installs for the current user. It does not require administrator access,
open firewall ports, or start before login.

## Choose Desktop or CLI

Spare Desktop is the default local experience. Each desktop archive contains
the application, daemon, CLI, built-in recipes, and uninstaller. Choose the
macOS package matching the processor:

```text
Intel Mac:         spare-desktop_0.1.1-alpha.3_darwin_amd64.zip
Apple Silicon Mac: spare-desktop_0.1.1-alpha.3_darwin_arm64.zip
```

Build and install it:

```bash
make desktop-package VERSION=0.1.1-alpha.3
cd dist/desktop
shasum -a 256 -c checksums.txt
unzip spare-desktop_0.1.1-alpha.3_darwin_$(test "$(uname -m)" = x86_64 && echo amd64 || echo arm64).zip
./install.sh
```

`make desktop-package` defaults to the current Mac. You can instead run
`make desktop-package-amd64` for Intel or `make desktop-package-arm64` for
Apple Silicon.

Spare opens immediately and initializes automatically; `spare init` is not
required. See [Use Spare Desktop](DESKTOP.md).

The desktop bundle is ad-hoc signed for local executable integrity. It has no
Developer ID signature and is not notarized.

A structurally verified, unsigned Windows amd64 engineering archive is also
created with:

```bash
make desktop-windows-package VERSION=0.1.1-alpha.3
```

Extract `spare-desktop_0.1.1-alpha.3_windows_amd64.zip` on Windows 11 and run its
`install.ps1`. It installs per user under
`%LOCALAPPDATA%\Programs\Spare`, opens the Wails app, and provides a Win32
system tray.

Linux desktop archives are built natively with
`make desktop-linux-package VERSION=0.1.1-alpha.3` after installing GTK3,
WebKitGTK, and Ayatana AppIndicator development packages. See
[Use Spare Desktop](DESKTOP.md) for exact dependencies and current native
acceptance gates.

Use the CLI archives below for ARM Windows, headless machines, Raspberry Pi,
or CLI-focused development.

## Choose the correct archive

| System | Intel/AMD 64-bit | ARM 64-bit |
| --- | --- | --- |
| macOS 13+ | `spare_0.1.1-alpha.3_darwin_amd64.tar.gz` | `spare_0.1.1-alpha.3_darwin_arm64.tar.gz` |
| Windows 11 | `spare_0.1.1-alpha.3_windows_amd64.zip` | `spare_0.1.1-alpha.3_windows_arm64.zip` |
| Ubuntu 22.04+, Debian 12+ | `spare_0.1.1-alpha.3_linux_amd64.tar.gz` | `spare_0.1.1-alpha.3_linux_arm64.tar.gz` |
| 64-bit Raspberry Pi OS | — | `spare_0.1.1-alpha.3_linux_arm64.tar.gz` |

Release files built from this repository are written to `dist/releases`.
Every platform archive includes the default `site_0.1.0.sp`,
`drop_0.1.0.sp`, and `hook_0.1.0.sp` packages in its `recipes` directory.
They are also published separately for direct download. Verify every selected
artifact against `dist/releases/checksums.txt` before installing or inspecting
it.

For an artifact published by the GitHub release workflow, also verify its
source-repository provenance with GitHub CLI:

```bash
gh attestation verify <downloaded-archive> --repo spare-run/spare
```

An attestation proves which repository and workflow produced an artifact. It
does not replace the missing macOS Developer ID or Windows code signature.

## Install on macOS

Extract the archive and run its installer:

```bash
tar -xzf spare_0.1.1-alpha.3_darwin_arm64.tar.gz
./install.sh
```

Use the `amd64` archive instead on an Intel Mac.

```bash
tar -xzf spare_0.1.1-alpha.3_darwin_amd64.tar.gz
./install.sh
```

The installer copies `spare` and `spared` to `~/.local/bin`, initializes Spare,
registers a LaunchAgent, and starts the daemon. If `~/.local/bin` is not already
on your path, add this line to `~/.zprofile`:

```bash
export PATH="$HOME/.local/bin:$PATH"
```

Then open a new terminal or run:

```bash
exec zsh -l
```

The preview is not signed or notarized. macOS may block a downloaded build. For
the safest development path, build it from source or approve only an archive
whose checksum you have verified.

The installer also creates `~/Applications/Spare Recipe Viewer.app` and
registers it for `.sp` files. Double-click a recipe package to inspect it, or
run `spare view package.sp`. The browser viewer is local to this Mac and does
not require the Spare daemon.

The bundled packages remain available after installation in
`~/Library/Application Support/Spare/recipes`.

## Install on Linux or Raspberry Pi

Extract the matching archive and run its installer:

```bash
tar -xzf spare_0.1.1-alpha.3_linux_amd64.tar.gz
./install.sh
```

Use `spare_0.1.1-alpha.3_linux_arm64.tar.gz` on an ARM64 computer or 64-bit Raspberry
Pi OS.

The installer copies both binaries to `~/.local/bin`, initializes Spare, and
enables a systemd user service. If needed, add the install directory to your
path:

```bash
export PATH="$HOME/.local/bin:$PATH"
```

Add that line to your shell profile to keep it after reopening the terminal.
Linux lingering is not enabled, so Spare starts after you log in and stops when
the user service manager is no longer running.

When the desktop MIME tools are available, the installer registers `.sp` as
`application/vnd.spare.recipe+zip` and associates it with Spare Recipe Viewer.
You can also run `spare view package.sp` directly.

The bundled packages remain available after installation in
`${XDG_STATE_HOME:-~/.local/state}/spare/recipes`.

## Install on Windows

Extract the selected ZIP archive. Open PowerShell in the extracted directory
and run:

```powershell
Set-ExecutionPolicy -Scope Process Bypass
.\install.ps1
```

The installer copies `spare.exe` and `spared.exe` to
`%LOCALAPPDATA%\Spare\bin`, adds that directory to your user `PATH`,
initializes Spare, and creates a logon Scheduled Task. Open a new terminal if
the `spare` command is not immediately available.

The preview binaries are unsigned. Windows may show a security warning. Verify
the archive checksum before approving it.

The installer registers the per-user `Spare.Recipe` file type. If `.sp` does
not already belong to another application, double-clicking it opens Spare
Recipe Viewer. `spare view package.sp` always works from a terminal.

The bundled packages remain available after installation in
`%LOCALAPPDATA%\Spare\recipes`.

## Build from source

Requirements:

- Go 1.25.12
- Node.js 24
- npm
- `make`

From the repository root:

```bash
make build
./bin/spare init
```

This registers `bin/spared` from the current repository location. Do not move
or delete the repository while using that development installation.

To build installable archives:

```bash
make release VERSION=0.1.1-alpha.3
```

## Confirm the installation

Run:

```bash
spare --version
spare status
spare recipe list
spare doctor
spare doctor --security
spare open dashboard
```

Before a recipe is installed, status should say:

```text
This computer is ready.
```

## State and service locations

| System | State and bundled recipes | Login service |
| --- | --- | --- |
| macOS | `~/Library/Application Support/Spare` | `~/Library/LaunchAgents/run.spare.spared.plist` |
| Windows | `%LOCALAPPDATA%\Spare` | Scheduled Task named `Spare` |
| Linux | `${XDG_STATE_HOME:-~/.local/state}/spare` | `${XDG_CONFIG_HOME:-~/.config}/systemd/user/spared.service` |

The state directory contains the SQLite database, API token, endpoint record,
installation record, and logs. Treat the API token as a private credential.

## Uninstall

From an extracted macOS or Linux archive:

```bash
./uninstall.sh
```

If only the installer is available:

```bash
./install.sh --uninstall
```

On Windows, run this from an extracted archive:

```powershell
.\uninstall.ps1
```

You can remove only Spare's state and login service with:

```bash
spare uninstall --yes
```

The platform uninstaller also removes the installed binaries. None of these
commands delete the folder that was served as a Site.
