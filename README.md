# Spare

**Spare gives any computer a job.**

This repository contains **Spare 0.1 Preview**, machine-readable version
`0.1.0`. Spare profiles the computer and runs one trusted recipe temporarily
or as a per-user background service.

Three built-in recipes prove the shared runtime:

- **Site** serves one folder as a read-only local website.
- **Drop** receives browser uploads into one selected folder and offers local
  download links.
- **Hook** receives, inspects, and replays webhook requests.

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
spare view drop.sp
spare recipe validate ./recipes/hook
```

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
- Recipe web interfaces have no authentication or TLS in this preview. Anyone
  on the same reachable local network can open Site, send files to Drop, or
  inspect requests and initiate replays through Hook.
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
POST   /api/v1/instances
POST   /api/v1/instances/{id}/start
POST   /api/v1/instances/{id}/stop
POST   /api/v1/instances/{id}/heartbeat
POST   /api/v1/browser-sessions
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
make recipes VERSION=0.1.0
make release VERSION=0.1.0
```

`make release` creates checksummed archives for macOS, Windows, and Linux on
amd64 and arm64, plus the Site, Drop, and Hook `.sp` packages, under
`dist/releases`.

This preview parses, validates, inspects, and packs `.sp` files, but executes
only the three trusted built-in implementations. It intentionally excludes
third-party artifact execution, containers, multiple simultaneous roles,
accounts, remote access, SpareOS, automatic updates, signing, notarization, and
telemetry.
