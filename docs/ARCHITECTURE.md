# Architecture

Spare `0.1.0` is a Go monorepo with CLI, daemon, and Wails desktop executables,
a generic recipe worker path, SQLite state, and one shared React application.

```text
Spare Desktop ─────┐
macOS menu bar ────┤
spare CLI ─────────┼── loopback JSON API ── spared
browser dashboard ─┘                         │
                                             ├── recipe registry
                                             ├── instance state
                                             ├── runtime drivers
                                             ├── shared supervisor
                                             ├── network and health
                                             └── Site, Drop, or Hook worker
```

Site, Drop, and Hook are ordinary clients of the same platform layers:

```text
Site ─┐
      ├── recipe → config → permissions → instance → runtime
Drop ─┤                                      │
Hook ─┘                                      │
                                             ├── network
                                             ├── health
                                             └── supervisor
```

## Executables

`spare` is the CLI. It initializes the current user, validates and packages
recipes, resolves configuration, calls the local API, maintains temporary
heartbeats, opens browser sessions, diagnoses problems, and exports or restores
data.

`spared` is the per-user daemon. It owns state, API/dashboard access, the recipe
registry, runtime drivers, supervision, health checks, and local discovery.

`spare-desktop` is the Wails shell and primary local control surface. Its Go
bridge performs automatic initialization, daemon recovery, authenticated API
calls, native folder selection, notifications, window control, menu-bar
actions, launch preferences, and uninstall. It does not own recipe lifecycle
state.

The React build detects its surface. Shared models and recipe controls remain
in one application, while the desktop frame exposes native actions and the
browser frame retains cookie sessions and remote-safe controls.

Child mode is generic:

```text
spared worker --recipe <id> --config <encoded-json> --port <port> --health-port <port>
```

The daemon dispatches the ID through the built-in registry. It does not contain
a Site-specific worker command.

## Packages

| Package | Responsibility |
| --- | --- |
| `internal/api` | Authenticated `/api/v1` server, CLI client, origin checks, and one-time browser sessions |
| `internal/artifacts` | Download, SHA-256 verification, safe extraction, platform selection, caching, and atomic package writes |
| `internal/auth` | Generate and load the 256-bit local API token |
| `internal/backup` | Export and safely restore instance configuration and selected-folder data |
| `internal/config` | Resolve typed string, directory, size, Boolean, and integer fields |
| `internal/dashboard` | Embed the Vite production build into `spared` |
| `internal/desktop` | Bound the Wails bridge, native dialogs, menu bar, notifications, preferences, and uninstall |
| `internal/discovery` | Advertise active recipes through best-effort mDNS |
| `internal/doctor` | Diagnose daemon, dashboard, service, recipe, port, folder, storage, LAN, and sleep state |
| `internal/health` | Serve and check shared worker health snapshots and metrics |
| `internal/instance` | Build configured instances from reusable recipe definitions |
| `internal/logs` | Rotate output at five 5 MB files |
| `internal/model` | Stable machine, recipe, instance, event, compatibility, and problem API types |
| `internal/network` | Allocate ports, discover LAN addresses, build endpoints, and normalize local hostnames |
| `internal/paths` | Resolve per-user state locations |
| `internal/preferences` | Persist launch preferences shared by desktop and daemon |
| `internal/permissions` | Describe declared filesystem, network, startup, and background access |
| `internal/profile` | Collect identity, CPU, memory, storage, LAN addresses, and capability signals |
| `internal/recipe` | Parse, validate, inspect, pack, compare, and register recipe manifests |
| `internal/recipes/site` | Implement the read-only static Site |
| `internal/recipes/drop` | Implement local browser upload and download |
| `internal/recipes/hook` | Implement bounded webhook capture, inspection, and replay |
| `internal/runtime` | Define the execution contract |
| `internal/runtime/native` | Run trusted built-in workers as native child processes |
| `internal/runtime/process` | Run one explicitly approved command without recipe-specific supervision code |
| `internal/service` | Generate and register LaunchAgent, systemd user, and Scheduled Task definitions |
| `internal/state` | Run SQLite migrations and persist machine, instance, and event state |
| `internal/supervisor` | Supervise runtimes, health, leases, mDNS, restart backoff, and crash limits |

## Recipe and instance distinction

A recipe is a reusable definition: identity, version, compatibility,
configuration fields, permissions, resources, network needs, storage, health,
and runtime.

An instance is one resolved installation: recipe ID/version, runtime, selected
folder, saved configuration, mode, desired state, status, port, URLs, metrics,
timestamps, and current problem.

The current one-role product constraint uses the recipe ID as the instance ID.
The state and API do not mix reusable recipe metadata with worker process state.

## Lifecycle

`spare init`:

1. Creates user-scoped state.
2. Generates the API token if missing.
3. Opens and migrates SQLite to schema version 2.
4. Profiles the computer and derives capabilities.
5. Registers the per-user login service.
6. Starts `spared` and waits for health.

Create flow:

1. Resolve a built-in ID or validate a `.sp` reference.
2. Check OS, architecture, memory, storage, network, and laptop warnings.
3. Resolve typed configuration and validate the selected folder.
4. Allocate a fixed or automatic port.
5. Prepare and start the selected runtime.
6. Monitor the shared health endpoint and publish URLs.

Installed desired state is persisted in SQLite. On daemon restoration, legacy
Site rows are migrated to versioned runtime/config/data fields before launch.

The daemon checks workers every 10 seconds and restarts after three failed
checks. Restart delays use capped exponential backoff. Five crashes within five
minutes leave the instance failed until a manual start.

Temporary instances require a lease heartbeat. The CLI owns it for `spare try`;
Spare Desktop owns it for visual temporary mode. They stop on Ctrl-C, an
explicit desktop quit choice, or within 15 seconds after the lease owner
disappears. A temporary instance can be promoted in place to installed mode.

## Package boundary

A `.sp` file is a reproducible ZIP-compatible archive containing `spare.yml`
and supporting files. V1 parsing uses known-field YAML validation, rejects
unsafe extraction paths and symlinks, and supports SHA-256 verification.

This preview intentionally executes only Site, Drop, and Hook implementations
compiled into `spared`. A valid package with any other ID can be inspected and
validated but cannot run. That boundary prevents the package feature from
becoming an arbitrary script runner before signing and isolation exist.
