# Use Spare Desktop

Spare Desktop is the primary local interface for the Desktop Alpha. The
browser dashboard remains available for phones, headless computers, and other
devices on the local network.

The first packaged desktop target is macOS 13 or newer on Apple Silicon. A
Windows amd64 ZIP, Win32 tray, and per-user PowerShell
installer are also built from this repository. Linux has a native
GTK/AppIndicator build and packaging path for amd64 and arm64. Windows and
Linux packages still require clean-machine acceptance passes before
publication.

## Install the macOS alpha

Build the package for the Mac you are currently using:

```bash
make desktop-package VERSION=0.1.0
cd dist/desktop
shasum -a 256 -c checksums.txt
unzip spare-desktop_0.1.0_darwin_$(test "$(uname -m)" = x86_64 && echo amd64 || echo arm64).zip
./install.sh
```

An Intel Mac requires `darwin_amd64`; an Apple Silicon Mac requires
`darwin_arm64`. To build either target explicitly on macOS:

```bash
make desktop-package-amd64 VERSION=0.1.0
make desktop-package-arm64 VERSION=0.1.0
```

The installer places `Spare.app` in `~/Applications`, links the advanced
`spare` and `spared` commands into `~/.local/bin`, and opens Spare.

The local build uses an ad-hoc signature so its embedded executables form a
valid macOS application bundle. It has no Developer ID signature and is not
notarized. Use it on a test account or a computer where you can remove it
safely. Do not bypass a macOS warning unless you built the package yourself or
verified its checksum.

## Install the Windows amd64 alpha

Build the ZIP from macOS or Linux with the Go cross-compiler and standard
`zip`, `file`, and `objdump` tools:

```bash
make desktop-windows-package VERSION=0.1.0
```

On Windows 11, extract
`spare-desktop_0.1.0_windows_amd64.zip`, then run:

```powershell
Set-ExecutionPolicy -Scope Process Bypass
.\install.ps1
```

The installer copies the GUI, daemon, CLI, built-in recipes, and uninstaller
to `%LOCALAPPDATA%\Programs\Spare`, adds its `bin` directory to the user
`PATH`, and opens Spare. The Windows system tray offers the same status,
sharing, lifecycle, activity, and quit actions as the macOS menu bar.

The executables are unsigned. Validate the checksum and use a test account or
clean VM.

## Build the Linux alpha

Build natively on Linux:

```bash
sudo apt-get install build-essential pkg-config libgtk-3-dev \
  libayatana-appindicator3-dev libwebkit2gtk-4.1-dev
make desktop-linux-package VERSION=0.1.0
```

If the distribution provides WebKitGTK 4.0 rather than 4.1, install
`libwebkit2gtk-4.0-dev`; the build script detects either version. The archive
name includes the native `amd64` or `arm64` architecture. Extract it and run
`./install.sh`.

## First launch

No terminal initialization is required. Opening Spare:

1. Creates protected per-user state and a 256-bit local API token.
2. Profiles the computer.
3. registers and starts the per-user daemon.
4. Connects the Go desktop layer to the authenticated loopback API.
5. Shows the built-in recipes.

The bearer token remains in Go. It is not placed in React storage or a browser
URL.

## Set up Drop

1. Select **Try Drop**.
2. Select **Choose folder** and choose where received files should be saved.
3. Set the maximum file size.
4. Choose **Keep running after login** or **Try temporarily**.
5. Select **Start Drop**.
6. Select **Share access** to show the LAN address and QR code.
7. Open that address on a phone or nearby computer and send a file.

The Home and Activity views update when the daemon records the received file.
Native notifications can also report received files and recipe problems.
Settings can enable notifications globally and independently for Drop, Site,
and Hook.

Files dragged onto an active Drop are copied into its selected folder through
the authenticated daemon API. A folder dragged onto an idle Spare opens Site
setup with that folder selected. Dropped `.sp` packages open in the safe
viewer, and dropped `.spare-backup` files open the restore workflow. A
received-file event offers **Show file**; the Go layer verifies that the
regular file still resolves inside the active Drop folder before revealing it
in Finder, File Explorer, or the Linux file manager.

A temporary recipe remains alive while Spare Desktop sends its lease
heartbeat. When quitting with a temporary recipe active, Spare offers:

- **Stop Drop and quit**
- **Keep Drop running**
- **Cancel**

Keeping it running promotes the same daemon instance to an installed instance.

## Menu bar and tray

The macOS menu bar and Windows/Linux system trays provide:

- Current recipe status
- Open recipe
- Show QR
- Pause or start the recipe
- Recent activity
- Open Spare
- Quit Spare

Closing the window hides it; it does not quit the menu-bar process. Use
**Quit Spare** when you want to end the desktop process.

## Settings

The three launch settings are independent:

- **Show Spare in the menu bar**
- **Open Spare after login**
- **Keep installed recipes running after login**

The daemon can restore an installed recipe without opening the full desktop
window. Notifications can be disabled separately.

Settings also contains per-recipe notifications, native backup export and
restore, the shared repair action, and a confirmed uninstall entry. **Open
remote dashboard** asks the daemon for a fresh single-use browser session and
opens it directly through the operating system; the permanent API credential
never enters React. **Recipe storage** shows the active recipe's selected
folder and provides native **Show selected folder** and **Choose another
folder** actions.
Uninstalling removes Spare state and the application but never deletes a Site
root, Drop destination, restored folder, or received file.

## Build for development

Build the current computer's desktop executable:

```bash
make desktop VERSION=0.1.0
./bin/spare-desktop
```

Build the production macOS bundle for the current Mac, or select a target
explicitly:

```bash
make desktop-package VERSION=0.1.0
make desktop-package-amd64 VERSION=0.1.0
make desktop-package-arm64 VERSION=0.1.0
make desktop-windows-package VERSION=0.1.0
# Run on Linux:
make desktop-linux-package VERSION=0.1.0
```

The resulting archive contains:

```text
Spare.app
├── Spare desktop executable
├── spared daemon
├── spare CLI
├── built-in Site, Drop, and Hook packages
└── uninstall helper
```

## Desktop Alpha limitations

- macOS amd64 can be exercised natively in the current Intel development
  environment. macOS ARM64 remains cross-built and requires the Apple Silicon
  acceptance pass.
- The Windows amd64 archive is cross-built and structurally verified, but its
  tray, PowerShell installer, login restart, and WebView2 behavior still need
  a native Windows 11 acceptance pass.
- Linux source includes GTK/AppIndicator tray and packaging support, but the
  archive must be built and exercised natively on Ubuntu and Debian.
- The build has only an ad-hoc local signature and is not notarized.
- Drop is local-network software without accounts, TLS, pairing, malware
  scanning, or remote internet access.
- Only one recipe can be active at a time.
- Recipe package inspection is safe preview only. Installing untrusted
  third-party recipe packages remains outside the trusted built-in boundary.

For the complete verification procedure, read [Test Spare](TESTING.md).
