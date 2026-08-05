# Spare

**Spare gives any computer a job.**

This repository contains **Spare 0.1 Preview**, machine-readable version
`0.1.1-alpha.3`. Spare profiles the computer and runs one trusted job
temporarily or as a per-user background service.

Three built-in recipes prove the shared runtime:

- **Site** serves one folder as a read-only local website.
- **Drop** receives browser uploads into one selected folder and offers local
  download links.
- **Hook** receives, inspects, and replays webhook requests.

The first-party optional-job catalog adds signed, metadata-only packages for
trusted implementations already compiled into Spare. **Clipboard**,
**Downloads**, and **Monitor** are available in the first wave. Archive,
Media, DNS, Ad Blocker, and Cameras are visible on the catalog roadmap but do
not become installable until their implementations ship.

The **Desktop Alpha** adds the primary local Wails interface:
automatic initialization, visual Drop setup, native folder selection, QR
sharing, live activity, menu-bar/system-tray controls, notifications,
drag-and-drop, backup/restore, settings, repair, and uninstall. macOS Intel is
natively exercised, macOS ARM64 and Windows amd64 are cross-built, and Linux
has a native build path pending clean-machine acceptance. The browser
dashboard remains the remote and headless surface.

Start with the [documentation index](docs/README.md) for installation, testing,
usage, repository structure, architecture, security, the project TODO, and the
original product notes.

For task-by-task instructions for Site, Drop, and Hook, read
[Use the built-in recipes](docs/BUILT-IN-RECIPES.md).

## Supported systems

- macOS 13 or newer
- Windows 11
- Ubuntu 22.04 or newer
- Debian 12 or newer
- 64-bit Raspberry Pi OS
- amd64 and arm64 architectures

Spare starts after the current user logs in. It does not require administrator privileges, enable Linux lingering, or modify the computer's firewall.

## Build

Requirements: Go 1.25.12, Node.js 24, npm, and `make`.

```bash
make build
make desktop
make desktop-package VERSION=0.1.1-alpha.3       # Current Mac architecture
make desktop-package-amd64 VERSION=0.1.1-alpha.3 # Intel Mac
make desktop-package-arm64 VERSION=0.1.1-alpha.3 # Apple Silicon
make desktop-windows-package VERSION=0.1.1-alpha.3
# On Linux:
make desktop-linux-package VERSION=0.1.1-alpha.3
```

The binaries are written to `bin/spare` and `bin/spared`.

Initialize a development build without changing your login services:

```bash
export SPARE_HOME="$(mktemp -d)"
export SPARED_PATH="$PWD/bin/spared"
export SPARE_NO_SERVICE=1
bin/spare init
```

## Use

```bash
spare init
spare try site ./public
spare try drop ./received-files
spare try hook
spare install site --path ./public
spare install drop --path ./received-files --max-file-size 2GB
spare install hook
spare job add ~/Downloads/clipboard_0.1.0.sp
spare install clipboard
spare status
spare open dashboard
spare open drop
spare open hook
spare stop drop
spare start drop
spare logs drop --follow
spare doctor
spare export drop
spare remove drop
spare job remove clipboard
spare uninstall
```

`spare try` remains attached to its terminal and expires within 15 seconds if
its heartbeat disappears. An installed recipe restarts after failures and
starts automatically after login.

Recipes bind to an automatically selected port from `7340–7399` unless a fixed
port is requested. Spare shows localhost, LAN IPv4, and best-effort `.local`
addresses. The local network may still require the user to approve an
operating-system firewall prompt.

Recipe development commands are available without installing a third-party
worker:

```bash
spare recipe validate ./recipes/drop
spare recipe pack ./recipes/drop
spare recipe inspect drop.sp
spare recipe sign clipboard.sp --key /secure/path/catalog-ed25519.pem \
  --minimum-spare-version 0.1.1-alpha.3
spare view drop.sp
spare recipe validate ./recipes/hook
```

Release archives include the bundled Site, Drop, and Hook `.sp` packages in a
`recipes` directory. Optional packages are downloaded separately from the job
catalog, reviewed on the local desktop, and copied into Spare's private job
library only after their Ed25519 signature and exact trusted manifest match
have been verified.

## Security boundary

- The dashboard and JSON API bind only to `127.0.0.1`, on the first available port from `7331–7339`.
- CLI requests use a private 256-bit bearer token.
- Browser access uses a one-time code and an HttpOnly, SameSite cookie. The long-lived token never appears in a browser URL.
- The Site recipe is read-only. It denies dotfiles, directory listings, traversal, and symlinks that resolve outside the selected folder.
- Drop accepts writes only through its upload endpoint, rejects unsafe names and
  symlinks, applies a per-file size limit, and resolves filename collisions.
- Hook keeps the latest 50 requests in memory, caps bodies at 1 MB, rejects
  cross-origin browser replays, and does not follow replay redirects.
- The `.sp` viewer binds to a random loopback port, validates every package
  path, renders text as inert content, and does not preview executables.
- Clipboard, Downloads, and Monitor require trusted-device pairing before
  their LAN interfaces can be used. Site, Drop, and Hook retain their
  documented trusted-network boundary in this preview.
- Optional `.sp` files contain metadata and assets, not executable plugin
  code. Spare executes only matching first-party implementations compiled into
  the installed release.
- Remove and uninstall commands never delete a recipe's selected folder.

## Local API

The versioned API is served from the loopback endpoint recorded in Spare's state directory:

```text
GET    /api/v1/health
GET    /api/v1/machine
GET    /api/v1/recipes
GET    /api/v1/instances
GET    /api/v1/instances/{id}
GET    /api/v1/events
GET    /api/v1/activity/stream
POST   /api/v1/instances
POST   /api/v1/instances/switch
POST   /api/v1/instances/{id}/start
POST   /api/v1/instances/{id}/stop
POST   /api/v1/instances/{id}/heartbeat
POST   /api/v1/instances/{id}/promote
POST   /api/v1/instances/{id}/configure
POST   /api/v1/browser-sessions
POST   /api/v1/desktop/backups/export
POST   /api/v1/desktop/backups/restore
POST   /api/v1/desktop/drop-files
GET    /api/v1/job-packages
POST   /api/v1/job-packages/review
POST   /api/v1/job-packages/install
DELETE /api/v1/job-packages/{id}
GET    /api/v1/job-profiles/{id}
DELETE /api/v1/instances/{id}
```

API errors use:

```json
{
  "error": {
    "code": "port_in_use",
    "message": "Port 7340 is already in use.",
    "hint": "Use `--port auto` or choose another port."
  }
}
```

## Test and release

```bash
make test
make test-ui
make smoke
make recipes VERSION=0.1.1-alpha.3
make catalog VERSION=0.1.1-alpha.3 \
  SPARE_CATALOG_SIGNING_KEY=/secure/path/catalog-ed25519.pem
make release VERSION=0.1.1-alpha.3
```

`make release` creates checksummed archives for macOS, Windows, and Linux on
amd64 and arm64, plus the Site, Drop, and Hook `.sp` packages, under
`dist/releases`.

`make desktop-package` separately creates the macOS application for the
current Mac architecture and its checksum under `dist/desktop`. Use the
architecture-specific targets for Intel or Apple Silicon. Read
[Use Spare Desktop](docs/DESKTOP.md) for its install and test flow.

This preview installs signed first-party optional-job packages but never
executes code from them. It intentionally excludes third-party artifact
execution, containers, multiple simultaneous active jobs, accounts, remote
access, SpareOS, automatic updates, application signing, notarization, and
telemetry.
