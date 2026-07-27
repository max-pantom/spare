# Install Spare

Spare `0.1.0` is an unsigned engineering preview. Install it only on a test
account or a computer where you can safely remove it.

Spare installs for the current user. It does not require administrator access,
open firewall ports, or start before login.

## Choose the correct archive

| System | Intel/AMD 64-bit | ARM 64-bit |
| --- | --- | --- |
| macOS 13+ | `spare_0.1.0_darwin_amd64.tar.gz` | `spare_0.1.0_darwin_arm64.tar.gz` |
| Windows 11 | `spare_0.1.0_windows_amd64.zip` | `spare_0.1.0_windows_arm64.zip` |
| Ubuntu 22.04+, Debian 12+ | `spare_0.1.0_linux_amd64.tar.gz` | `spare_0.1.0_linux_arm64.tar.gz` |
| 64-bit Raspberry Pi OS | — | `spare_0.1.0_linux_arm64.tar.gz` |

Release files built from this repository are written to `dist/releases`.
That directory also contains the optional `site_0.1.0.sp`,
`drop_0.1.0.sp`, and `hook_0.1.0.sp` packages. Verify every selected artifact
against `dist/releases/checksums.txt` before installing or inspecting it.

## Install on macOS

Extract the archive and run its installer:

```bash
tar -xzf spare_0.1.0_darwin_arm64.tar.gz
./install.sh
```

Use the `amd64` archive instead on an Intel Mac.

```bash
tar -xzf spare_0.1.0_darwin_amd64.tar.gz
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

## Install on Linux or Raspberry Pi

Extract the matching archive and run its installer:

```bash
tar -xzf spare_0.1.0_linux_amd64.tar.gz
./install.sh
```

Use `spare_0.1.0_linux_arm64.tar.gz` on an ARM64 computer or 64-bit Raspberry
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
make release VERSION=0.1.0
```

## Confirm the installation

Run:

```bash
spare --version
spare status
spare recipe list
spare doctor
spare open dashboard
```

Before a recipe is installed, status should say:

```text
This computer is ready.
```

## State and service locations

| System | State | Login service |
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
