<p align="center">
  <img src="desktop/icons/app-icon.svg" width="96" height="96" alt="Spare logo">
</p>

<h1 align="center">Spare</h1>

<p align="center">
  <strong>Give a computer one useful job.</strong><br>
  Run local tools for sharing files, serving a site, receiving webhooks, and
  more—without turning the computer into a server project.
</p>

<p align="center">
  <a href="docs/INSTALLATION.md">Install</a> ·
  <a href="docs/DESKTOP.md">Use the desktop app</a> ·
  <a href="docs/SECURITY.md">Security</a> ·
  <a href="docs/README.md">Documentation</a> ·
  <a href="TODO.md">Roadmap</a>
</p>

> [!WARNING]
> Spare is an engineering preview (`0.1.1-alpha.3`). The macOS and Windows
> builds are not signed for public distribution. Apple Silicon has completed
> the main native acceptance pass; Intel macOS, Windows, and Linux still need
> clean-machine testing. Build it yourself or use it on a test computer with a
> trusted local network.

## What is Spare?

Spare turns a Mac, Windows PC, or Linux computer into a focused local service.
Choose a job, review what it needs, select any required folder, and start it.
Spare keeps the job running, shows its health and activity, and gives you a QR
code when another device can connect.

The desktop app is the main control surface. A browser dashboard is available
when you are accessing the computer from another device, and the CLI provides
the same lifecycle controls for development and headless machines.

Spare currently runs one active job at a time. You can install and configure
several jobs, then switch between them without losing their settings.

## Available jobs

| Job | What it does | Availability | Network access |
| --- | --- | --- | --- |
| **Site** | Serves a selected folder as a read-only website | Built in | Trusted LAN |
| **Drop** | Receives files from nearby browsers into a selected folder | Built in | Trusted LAN |
| **Hook** | Captures, inspects, and safely replays webhook requests | Built in | Trusted LAN |
| **Clipboard** | Shares expiring text, links, and small files between paired devices | Optional | Paired devices |
| **Downloads** | Downloads direct HTTP or HTTPS file links into a selected folder | Optional | Internet + paired devices |
| **Monitor** | Checks websites, hosts, and TCP ports and records their status | Optional | Internet/LAN + paired devices |

Optional jobs arrive as signed `.sp` packages. In this preview, a package can
enable only an exact implementation already compiled into Spare. It cannot add
or execute an arbitrary third-party binary.

Downloads accepts a direct file URL—the address that returns the file itself.
It does not extract videos or files from download pages, streaming sites, or
HTML landing pages.

[Read how optional jobs work](docs/JOBS.md) or
[follow the built-in job guides](docs/BUILT-IN-RECIPES.md).

## How it works

```text
Spare Desktop ─────┐
menu bar / tray ───┤
spare CLI ─────────┼── authenticated loopback API ── spared daemon
browser dashboard ─┘                                  │
                                                       ├── job supervisor
                                                       ├── health + activity
                                                       ├── SQLite state
                                                       └── isolated job worker
```

- **Go** provides the daemon, CLI, job implementations, native desktop bridge,
  security boundaries, and platform integration.
- **React and TypeScript** provide the shared desktop and browser interface.
- **Wails** places the React interface inside a native desktop window.
- **SQLite** stores local configuration, job state, and activity.
- **Vite** builds the frontend bundled into the Go application.

The UI does not control worker processes directly. It talks to the local Go
daemon through an authenticated API bound to `127.0.0.1`. The daemon owns job
lifecycle, recovery, health checks, ports, local discovery, and persistent
state.

[Read the architecture guide](docs/ARCHITECTURE.md).

## Try the desktop app

### macOS

Requirements: macOS 13 or newer, Go 1.25.12, Node.js 24, npm, and `make`.

```bash
git clone https://github.com/max-pantom/spare.git
cd spare
make desktop-package VERSION=0.1.1-alpha.3
cd dist/desktop
shasum -a 256 -c checksums.txt
unzip spare-desktop_0.1.1-alpha.3_darwin_$(test "$(uname -m)" = x86_64 && echo amd64 || echo arm64).zip
./install.sh
```

The installer places `Spare.app` in `~/Applications` and installs the daemon,
CLI, bundled jobs, and uninstaller for the current user. No administrator
access is required.

The app has only an ad-hoc local signature. Do not bypass a macOS warning for
an archive you did not build or verify.

### Windows and Linux

Windows 11 amd64 and Linux amd64/arm64 packaging are implemented, but their
clean-machine acceptance passes are not complete. Use a VM or test computer.

```bash
# Cross-build the Windows 11 amd64 package
make desktop-windows-package VERSION=0.1.1-alpha.3

# Run natively on Linux after installing GTK, WebKitGTK, and AppIndicator
make desktop-linux-package VERSION=0.1.1-alpha.3
```

[Read the complete installation guide](docs/INSTALLATION.md) for archive names,
dependencies, checksum verification, per-platform installation, and removal.

## Try the CLI without installing a login service

This isolated development setup keeps all state in a temporary directory and
does not register a startup service:

```bash
make build
export SPARE_HOME="$(mktemp -d)"
export SPARED_PATH="$PWD/bin/spared"
export SPARE_NO_SERVICE=1
bin/spare init

mkdir -p received-files
bin/spare try drop ./received-files
```

While the command is running, open the address it prints from a nearby device.
Stopping the command stops the temporary job. Installed jobs instead recover
after failures and can start after login.

Useful commands:

```bash
spare status
spare open dashboard
spare start drop
spare stop drop
spare logs drop --follow
spare doctor
spare doctor --security
spare export drop --output drop.spare-backup
spare remove drop
spare uninstall
```

[Read the CLI reference](docs/USAGE.md).

## Security model

Spare is local-first, but local does not automatically mean private. Understand
the boundary before exposing a job to another device:

- The management API and dashboard bind only to `127.0.0.1` and require a
  private 256-bit token or a short-lived browser session.
- Site, Drop, and Hook have no accounts or TLS in this preview. Every device
  that can reach their port must be treated as trusted.
- Clipboard, Downloads, and Monitor require a six-digit pairing flow and use
  bounded sessions, but their normal LAN connection is still HTTP.
- macOS workers use deny-by-default sandbox profiles. Linux workers use
  Landlock filesystem rules. Windows uses protected state ACLs and restricted
  Job Objects; per-worker AppContainer isolation remains unfinished.
- Selected folders are never deleted when a job or Spare itself is removed.
- Spare does not configure the firewall, open router ports, create a public
  tunnel, provide cloud storage, or collect telemetry.

Do not expose Spare through port forwarding, a public tunnel, or a public cloud
firewall rule. Drop does not scan received files for malware.

[Review the complete security boundary](docs/SECURITY.md).

## Platform status

| Platform | Build status | Native acceptance status |
| --- | --- | --- |
| macOS 13+ Apple Silicon | Desktop package available | Main install, login, tray, notification, and uninstall pass complete |
| macOS 13+ Intel | Desktop package available | Clean Intel Mac pass pending |
| Windows 11 amd64 | Cross-build available | Installer, WebView2, tray, login, ACL, and uninstall pass pending |
| Linux amd64/arm64 | Native build path available | Ubuntu, Debian, and ARM64 passes pending |
| Windows 11 ARM64 | CLI release target | Desktop acceptance not complete |
| 64-bit Raspberry Pi OS | CLI release target | Clean-device pass pending |

## Preview limitations

The current preview intentionally does not include:

- Publicly trusted application signing or macOS notarization
- Automatic or signed application updates
- Multiple simultaneously active jobs
- Remote internet access or Spare-hosted accounts
- Arbitrary third-party executable jobs
- Containers, virtual machines, or SpareOS
- Encrypted backups or encrypted job state
- Built-in malware scanning for received files

See [the project TODO](TODO.md) for the verification gates and remaining
security work.

## Develop and test

```bash
make build       # Build the dashboard, CLI, and daemon
make desktop     # Build the native desktop executable
make test        # Build the dashboard, run Go tests, and run go vet
make test-ui     # Run Playwright desktop/browser tests
make smoke       # Exercise Site, Drop, Hook, backup, and removal end to end
make schema      # Regenerate the API schema and endpoint reference
```

The repository is organized as a Go monorepo with the React application in
`dashboard/`, native desktop packaging in `desktop/`, built-in and optional job
definitions in `recipes/`, and longer guides in `docs/`.

- [Documentation index](docs/README.md)
- [Repository structure](docs/FILE-STRUCTURE.md)
- [API reference](docs/API.md)
- [Recipe package format](docs/RECIPES.md)
- [Testing guide](docs/TESTING.md)
- [Backup and restore](docs/BACKUP.md)
