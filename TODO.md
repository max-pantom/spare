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

## Before the `0.1.0` tag

- [ ] Run the complete install, login, Site, Drop, Hook, export, removal, and
  uninstall flow on a clean macOS 13+ Intel machine.
- [ ] Run the same flow on Apple Silicon.
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

- [ ] Recover cleanly from a corrupt or partially written SQLite database.
- [ ] Add integration coverage for LAN changes while a recipe is running.
- [ ] Record Drop transfers as structured daemon activity without weakening the
  worker boundary.
- [ ] Add clearer guidance when the operating-system firewall blocks LAN
  access.
- [ ] Add non-ASCII hostname and mDNS conflict-resolution coverage.
- [ ] Generate stable JSON schema and API reference documentation.
- [ ] Add a support bundle that excludes API tokens, backups, and selected
  folder contents.
- [ ] Add encrypted backups or explicit integration with an operating-system
  secret/storage provider.
- [ ] Add malware-scanning hooks before positioning Drop for untrusted
  networks.

## External recipes

- [ ] Define publisher identity, signatures, provenance, and revocation.
- [ ] Pin and verify packaged binary checksums from the manifest.
- [ ] Enforce declared permissions at an operating-system isolation boundary.
- [ ] Add an installation review and consent flow for third-party recipes.
- [ ] Add versioned artifact caching, cleanup, upgrades, and rollback.
- [x] Run a third recipe without adding recipe-specific platform code. Hook
  demonstrates this platform acceptance test.

Until these are complete, `.sp` packages with IDs other than `site`, `drop`,
or `hook` can be validated and inspected but cannot execute.

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
