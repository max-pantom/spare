# Use Spare

Spare `0.1.0` gives one computer one primary recipe role. Site, Drop, and Hook
use the same instance, runtime, networking, health, supervision, and lifecycle
systems.

For task-by-task setup and safety instructions, read
[Use the built-in recipes](BUILT-IN-RECIPES.md).

## Choose a recipe

Site serves a folder as a read-only local website:

```bash
spare try site ./public
spare install site --path ./public --port auto
```

Drop receives browser uploads into a selected folder:

```bash
spare try drop ./received-files
spare install drop --path ./received-files --max-file-size 2GB
```

Hook receives, inspects, and replays local webhook requests:

```bash
spare try hook
spare install hook
```

Temporary recipes stop on Ctrl-C or within 15 seconds after the attached CLI
heartbeat disappears. Installed recipes restart after failures and return
after you log in.

## Common workflow

Initialize Spare once:

```bash
spare init
```

See compatible built-in recipes:

```bash
spare recipe list
```

Inspect the current role:

```bash
spare status
spare status --json
spare open dashboard
spare open recipe
```

Control the installed recipe by ID:

```bash
spare stop drop
spare start drop
spare logs drop
spare logs drop --follow
spare doctor
```

Remove the role without deleting its selected folder:

```bash
spare remove drop
```

Remove Spare from the current user account:

```bash
spare uninstall
```

## Commands

| Command | Purpose |
| --- | --- |
| `spare init` | Profile the computer, initialize state, register the login service, and start the daemon |
| `spare recipe list` | List built-in recipes and compatibility ratings |
| `spare recipe validate <source>` | Validate a recipe directory, manifest, or `.sp` package |
| `spare recipe pack <directory>` | Create a reproducible ZIP-compatible `.sp` package |
| `spare recipe inspect <source>` | Print a validated manifest as JSON |
| `spare view <package.sp>` | Open a validated package summary, file list, and safe previews in a local browser |
| `spare try <recipe> [directory]` | Run a recipe while the CLI heartbeat remains active |
| `spare install <recipe> --path <directory>` | Install one persistent recipe instance |
| `spare status [--json]` | Show machine, recipe, instance, addresses, metrics, and problems |
| `spare open dashboard` | Create a one-time browser session and open the dashboard |
| `spare open <recipe>` | Open the installed recipe |
| `spare start <recipe>` | Set the installed instance's desired state to running |
| `spare stop <recipe>` | Set the installed instance's desired state to stopped |
| `spare logs <recipe> [--follow]` | Read or follow its rotating worker log |
| `spare doctor [--json]` | Check daemon, dashboard, service, health, port, folder, storage, LAN, and sleep risks |
| `spare export <recipe>` | Export configuration and selected-folder data |
| `spare import <backup> --path <empty-directory>` | Restore data and install the saved recipe |
| `spare remove <recipe> [--yes]` | Remove instance metadata and logs, never selected-folder data |
| `spare uninstall [--yes]` | Stop Spare, unregister its login service, and remove user state |

## Port selection

The dashboard and management API bind only to loopback. They prefer port
`7331` and scan through `7339`.

Recipes use the first available port from `7340` through `7399`:

```bash
spare try drop ./received-files --port auto
```

Request a fixed port when another tool depends on it:

```bash
spare install site --path ./public --port 7350
```

A fixed-port collision fails with a recovery hint. An automatic port may be
reassigned and recorded as an activity event.

## Instance rules

Only one primary recipe instance can exist.

- Installing identical configuration again is a no-op.
- Changing the recipe, selected folder, file-size limit, or fixed port requires
  removal followed by a new installation.
- Start and stop are idempotent.
- Removal and uninstall never delete the selected folder.

The current implementation uses the recipe ID as the single instance ID. The
separate recipe and instance models allow named or multiple instances later
without moving recipe metadata into process state.

## Site behavior

- Resolves the selected root to an absolute canonical path.
- Serves `index.html` for directories.
- Disables directory listings.
- Denies dotfiles and traversal.
- Allows symlinks only when the resolved target remains inside the root.
- Reflects file changes on the next request.
- Has no SPA fallback or live reload.

## Drop behavior

- Receives one browser upload at a time.
- Shows browser upload progress.
- Offers received files as download links.
- Reports received-file count and available storage.
- Rejects files above `--max-file-size`, which defaults to `2GB`.
- Renames collisions as `name (1).ext`, `name (2).ext`, and so on.
- Rejects hidden names, unsafe path characters, and symlinks.
- Writes only inside the selected destination folder.

Drop has no accounts, pairing, TLS, sync, cloud storage, or remote access in
this preview. Treat its LAN URL as writable by every device that can reach it.

## Hook behavior

- Receives any HTTP method at `/hook` and `/hook/*`.
- Shows the method, path, query, source address, headers, and body.
- Keeps the newest 50 requests in memory while Hook is running.
- Rejects request bodies larger than 1 MB.
- Replays the original method, end-to-end headers, and body to a full HTTP or
  HTTPS URL.
- Records replay status, duration, destination response headers, and up to 64
  KB of the response body.
- Keeps the latest 20 replay attempts for each captured request.
- Does not follow redirects when replaying, which prevents captured credentials
  from being forwarded to another redirect destination.
- Rejects cross-origin browser replay actions.

Hook history is intentionally temporary and cannot be exported. It has no
accounts or TLS in this preview. Anyone who can reach Hook can inspect captured
secrets and initiate a replay, so use it only on a trusted network.

## Network visibility

Spare reports:

- A localhost URL for the Spare computer
- Non-loopback IPv4 URLs for nearby devices
- A conflict-resolved `.local` URL when mDNS succeeds

mDNS failure does not make a recipe unhealthy. LAN access can still be blocked
by the host firewall, Wi-Fi client isolation, VPN settings, or the network
configuration.
