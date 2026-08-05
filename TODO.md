# Spare TODO

This list records completed platform work, pre-tag verification, and features
that remain deliberately outside the trusted built-in V1 boundary.

## Platform foundation completed

- [x] Move Site from `internal/site` to `internal/recipes/site`.
- [x] Dispatch generic child workers through a recipe registry.
- [x] Separate recipe definitions from configured instance state.
- [x] Add native and approved-process runtime interfaces.
- [x] Move port selection, LAN endpoints, and local hostnames into
  `internal/network`.
- [x] Add shared health endpoints, metrics, and checks.
- [x] Add typed configuration and declared permission schemas.
- [x] Add capability-based machine and recipe compatibility.
- [x] Parse, validate, inspect, and reproducibly pack `.sp` recipes.
- [x] Add a safe local `.sp` viewer with package previews and per-user desktop
  file associations.
- [x] Add SHA-256 verification, safe extraction, platform artifact selection,
  caching paths, downloads, and atomic replacement.
- [x] Add the signed first-party job catalog. Optional `.sp` packages contain
  metadata only and can unlock only an exact trusted implementation already
  compiled into the same Spare release.
- [x] Bound optional package size and contents, stage installation in private
  storage, and reverify the installed copy before making a job available.
- [x] Bound Clipboard and Monitor storage, queues, histories, sessions, request
  sizes, redirects, and persisted state for the first downloadable job wave.
- [x] Add Drop with browser upload/download, progress, size limits, available
  storage, collision-safe names, and selected-folder preservation.
- [x] Add Hook with bounded in-memory request history, detailed inspection,
  safe replay controls, and a local browser interface.
- [x] Expand `spare doctor` across daemon, dashboard, service, recipe, network,
  storage, and laptop sleep checks.
- [x] Add export and safe restore for configuration and selected-folder data.
- [x] Generalize the dashboard into Instance, Recipes, Machine, and Activity
  views.
- [x] Add SQLite schema version 2 and migration of legacy Site instance JSON.

## Desktop Alpha

- [x] Add the Wails shell without duplicating the React application.
- [x] Share initialization and daemon recovery between the CLI and desktop.
- [x] Keep daemon authentication in Go instead of browser storage.
- [x] Add manifest-driven visual recipe setup and a native folder picker.
- [x] Make the desktop process own temporary recipe heartbeats and promotion.
- [x] Add live activity streaming and structured Drop receipt events.
- [x] Add QR sharing, native notifications, and hardened macOS menu-bar
  controls with the Spare mark, live Drop progress, and recovery states.
- [x] Add separate menu-bar, desktop-login, and recipe-restore preferences.
- [x] Add settings, repair, safe removal, and uninstall entry points.
- [x] Build a single macOS ARM64 bundle containing the GUI, CLI, daemon,
  built-in recipes, and uninstaller.
- [x] Add native package/backup pickers, drag-and-drop routing, active recipe
  reconfiguration, and per-recipe notification preferences.
- [x] Cross-build the Windows amd64 Wails shell, Win32 system tray, per-user
  installer, and uninstaller as one checksummed archive.
- [x] Add the native Linux GTK/AppIndicator tray, per-user installer, and
  amd64/arm64 packaging path.
- [x] Complete a hands-on Apple Silicon install, notification, menu-bar, login
  restart, and uninstall acceptance pass before publishing the Desktop Alpha.
- [ ] Validate the Windows amd64 archive, tray, WebView2 shell, login restart,
  and uninstall on Windows 11.
- [ ] Build and validate Linux desktop archives on Ubuntu 22.04+, Debian 12+,
  and ARM64 Linux after installing the native WebKitGTK/AppIndicator toolchain.

## Before the `0.1.0` tag

- [ ] Run the complete install, login, Site, Drop, Hook, export, removal, and
  uninstall flow on a clean macOS 13+ Intel machine.
- [x] Run the same flow on Apple Silicon.
- [ ] Run the same flow on Windows 11 amd64.
- [ ] Run the same flow on Windows 11 ARM64.
- [ ] Run the same flow on Ubuntu 22.04+ and Debian 12+.
- [ ] Run the same flow on 64-bit Raspberry Pi OS.
- [ ] Confirm that each installed recipe returns after logout and login on
  every supported system.
- [ ] Test install, selected-folder, filename, package, and backup paths
  containing spaces and Unicode.
- [ ] Test fixed and automatic port collisions on every operating system.
- [ ] Test a missing or unmounted selected folder and confirm the dashboard and
  doctor give useful recovery instructions.
- [ ] Test LAN address changes, blocked mDNS, host-firewall blocking, and Wi-Fi
  client isolation.
- [ ] Force repeated Site, Drop, and Hook crashes and confirm the
  five-crashes-in-five-minutes limit requires a manual start.
- [ ] Exercise each installer and uninstaller in a clean VM.
- [ ] Test Drop with large, empty, Unicode, duplicate, and intentionally
  malformed filenames.
- [ ] Test backup and restore with a large directory and interrupted writes.
- [ ] Review GitHub Actions for the tagged commit.
- [ ] Publish six executable archives, three `.sp` packages, and
  `checksums.txt`.

## Harden the engineering preview

- [x] Harden local credentials, endpoint discovery, private state paths, login
  service definitions, and uninstall liveness checks.
- [x] Add `spare doctor --security` with explicit package, network, service,
  state-permission, executable-integrity, and worker-isolation reporting.
- [x] Pin CI actions and add race detection, Go vulnerability analysis,
  CodeQL, dependency review, and release provenance attestations.
- [ ] Enforce and verify owner-only Windows ACLs for credentials, endpoint
  state, logs, packages, and job data instead of relying only on inherited
  `%LOCALAPPDATA%` permissions.
- [ ] Put built-in workers behind platform filesystem and process isolation
  while preserving explicitly selected folders and required local-network
  access.
- [x] Preserve a corrupt or partially written SQLite database, create fresh
  state, and record the recovery in Activity.
- [ ] Add integration coverage for LAN changes while a recipe is running.
- [x] Record Drop transfers as structured daemon activity without weakening the
  worker boundary.
- [x] Add clearer guidance when the operating-system firewall may block LAN
  access.
- [x] Add bounded non-ASCII hostname handling and stable per-machine mDNS
  service names that avoid common name conflicts.
- [x] Generate stable JSON Schema and API endpoint reference documentation.
- [x] Add a support bundle that excludes API tokens, identity, network
  addresses, configuration, logs, backups, and selected
  folder contents.
- [ ] Add encrypted backups or explicit integration with an operating-system
  secret/storage provider.
- [ ] Add malware-scanning hooks before positioning Drop for untrusted
  networks.

## Third-party recipes

- [ ] Define publisher identity, signatures, provenance, and revocation.
- [ ] Pin and verify packaged binary checksums from the manifest.
- [ ] Enforce declared permissions at an operating-system isolation boundary.
- [ ] Add an installation review and consent flow for third-party recipes.
- [ ] Add versioned artifact caching, cleanup, upgrades, and rollback.
- [x] Run a third recipe without adding recipe-specific platform code. Hook
  demonstrates this platform acceptance test.

Until these are complete, an arbitrary `.sp` package can be validated and
inspected but cannot supply executable code. The optional-job installer accepts
only Spare-signed metadata whose manifest exactly matches a trusted
implementation already compiled into that Spare release.

## Release engineering after the preview

- [ ] Sign Windows executables and installers.
- [ ] Sign and notarize macOS binaries.
- [ ] Add a signed update channel and rollback strategy.
- [ ] Add Homebrew distribution.
- [ ] Add Winget distribution.
- [ ] Define release support and compatibility policies.

## Deferred product work

- [ ] Add managed runtimes only when they can be isolated and updated safely.
- [ ] Evaluate containers without exposing container setup to ordinary users.
- [ ] Prototype SpareOS on a stable Linux base after Hosted mode is proven.
- [ ] Design opt-in remote access separately from local discovery.
- [ ] Design multiple simultaneous roles after one-role reliability is proven.
- [ ] Define privacy-preserving, opt-in diagnostics before adding telemetry.

The product direction behind these items is preserved in
[`docs/product-notes`](docs/product-notes).
