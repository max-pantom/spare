# Spare

**Spare gives any computer a job.**

This repository contains the `0.1.0` Hosted-mode engineering preview. It can serve one folder as a read-only website, temporarily or as a per-user background service, and manage it through a local CLI and dashboard.

## Supported systems

- macOS 13 or newer
- Windows 11
- Ubuntu 22.04 or newer
- Debian 12 or newer
- 64-bit Raspberry Pi OS
- amd64 and arm64 architectures

Spare starts after the current user logs in. It does not require administrator privileges, enable Linux lingering, or modify the computer's firewall.

## Build

Requirements: Go 1.25.6, Node.js 24, npm, and `make`.

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
spare install site --path ./public
spare status
spare open dashboard
spare open site
spare stop site
spare start site
spare logs site --follow
spare doctor
spare remove site
spare uninstall
```

`spare try` remains attached to its terminal and expires within 15 seconds if its heartbeat disappears. An installed Site restarts after failures and starts automatically after login.

Site binds to an automatically selected port from `7340–7399` unless a fixed port is requested. Spare shows localhost, LAN IPv4, and best-effort `.local` addresses. The local network may still require the user to approve an operating-system firewall prompt.

## Security boundary

- The dashboard and JSON API bind only to `127.0.0.1`, on the first available port from `7331–7339`.
- CLI requests use a private 256-bit bearer token.
- Browser access uses a one-time code and an HttpOnly, SameSite cookie. The long-lived token never appears in a browser URL.
- The Site recipe is read-only. It denies dotfiles, directory listings, traversal, and symlinks that resolve outside the selected folder.
- The Site itself has no authentication or TLS in this preview. Anyone on the same reachable local network can open its address.
- `spare remove site` and `spare uninstall` never delete the served folder.

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
make release VERSION=0.1.0
```

`make release` creates checksummed archives for macOS, Windows, and Linux on amd64 and arm64 under `dist/releases`.

This preview intentionally excludes `.sp` packages, containers, multiple roles, Drop, SpareOS, remote access, automatic updates, signing, notarization, and telemetry.
