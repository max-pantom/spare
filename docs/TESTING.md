# Test Spare

Use the automated smoke test first. It creates isolated temporary state, does
not register a login service, and removes its test data when it finishes.

## Run the automated checks

Install the dashboard dependencies and browser once:

```bash
cd dashboard
npm ci
npx playwright install chromium
cd ..
```

Then run:

```bash
make test
make test-ui
make smoke
```

Expected results:

- `make test` builds the dashboard, runs every Go test, and runs `go vet`.
- `make test-ui` runs seven Chromium tests, including the desktop first-launch
  Drop flow, axe accessibility checks, keyboard access, 320 px reflow, and
  200% text scaling.
- `make smoke` builds Spare and exercises all three recipes through the shared
  runtime. It tests Site serving and worker recovery, Drop upload/download,
  Hook capture/replay, stop/start, removal, uninstall, backup/restore, and
  selected-folder preservation.

Run the vulnerability and dependency checks separately:

```bash
go run golang.org/x/vuln/cmd/govulncheck@latest ./...
cd dashboard && npm audit
```

Build and verify all release archives and recipe packages:

```bash
make release VERSION=0.1.0
cd dist/releases
shasum -a 256 -c checksums.txt
```

On Linux, use `sha256sum -c checksums.txt` if `shasum` is unavailable.

## Test the macOS Desktop Alpha

Build and verify the archive for the current Mac:

```bash
make desktop-package VERSION=0.1.0
cd dist/desktop
shasum -a 256 -c checksums.txt
unzip spare-desktop_0.1.0_darwin_$(test "$(uname -m)" = x86_64 && echo amd64 || echo arm64).zip
./install.sh
```

Use `make desktop-package-amd64` explicitly for Intel and
`make desktop-package-arm64` for Apple Silicon. On the matching Mac, verify
this complete path:

1. Spare opens without running `spare init`.
2. The first screen shows the machine profile and **Try Drop**.
3. The native folder picker selects a destination containing spaces or
   non-ASCII characters.
4. **Start Drop** produces a ready status, LAN address, and QR code.
5. A phone on the same network uploads a file.
6. Home updates the received-file count and Activity names the received file.
7. A native notification appears when notifications are enabled.
8. Menu-bar Open, Show QR, Pause, Start, Activity, and Open Spare actions work.
9. Closing the window leaves the menu-bar process active.
10. Quitting a temporary Drop offers stop, promote, and cancel choices.
11. An installed Drop returns after logging out and back in.
12. Disabling **Keep installed recipes running after login** prevents that
    automatic restore while preserving the installed configuration.
13. Uninstall removes the app, daemon service, desktop login item, and Spare
    state while leaving every received file unchanged.
14. Drag regular files onto an active Drop and confirm the daemon copies them
    with collision-safe names and updates Activity.
15. Drag a folder onto an idle Spare and confirm Site setup opens with the
    canonical folder selected.
16. Drag a `.sp` package and confirm only the safe package viewer opens.
17. Export a backup in Settings, remove the current job, drag the backup into
    Spare, restore it to an empty folder, and confirm the installed state.
18. Configure the active recipe with another folder and confirm the former
    folder remains unchanged.
19. In Settings, select **Open remote dashboard** and confirm the browser
    receives a working single-use session without a bearer token in its URL.
20. In **Recipe storage**, select **Show selected folder**, then **Choose
    another folder**, and confirm both actions stay on the local desktop
    surface.
21. Receive a Drop file, select **Show file** on its activity entry, and
    confirm the operating system reveals that exact file. Replace it with a
    symlink outside the Drop folder and confirm Spare refuses to reveal it.
22. Disable only Hook notifications and confirm Drop notifications remain
    enabled.

The automated desktop test uses the same React surface with a bounded Wails
bridge mock. Native folder dialogs, menu-bar interaction, notifications,
LaunchAgents, login restart, and Finder removal still require this hands-on
macOS pass.

## Test the Windows Desktop Alpha

Build the archive and verify its checksum:

```bash
make desktop-windows-package VERSION=0.1.0
cd dist/desktop
shasum -a 256 -c checksums.txt
```

In a clean Windows 11 amd64 VM, extract
`spare-desktop_0.1.0_windows_amd64.zip`, run `install.ps1`, and repeat the
complete desktop path above. Also verify:

- `Spare.exe` and `spared.exe` are GUI-subsystem executables while
  `bin\spare.exe` remains a console CLI.
- The Win32 tray opens, shows current status, opens the recipe and QR view,
  pauses/starts the recipe, opens Activity, and quits.
- All three startup choices remain independent after logoff and login.
- `uninstall.ps1` removes the application, user PATH entry, scheduled task,
  and file association without deleting selected folders.

Cross-compilation and archive inspection do not replace this native pass.

## Test the Linux Desktop Alpha

On Ubuntu or Debian, install the native build dependencies listed in
[Use Spare Desktop](DESKTOP.md), then run:

```bash
make desktop-linux-package VERSION=0.1.0
cd dist/desktop
shasum -a 256 -c checksums.txt
```

Extract the matching `linux_amd64` or `linux_arm64` archive and run
`./install.sh`. Repeat the desktop path above and verify the
GTK/AppIndicator tray under both X11 and Wayland sessions. Run the same
acceptance flow on Ubuntu 22.04+, Debian 12+, and an ARM64 Linux computer.

## Test a temporary Site

Install Spare first, then create a folder containing an `index.html`.

On macOS or Linux:

```bash
mkdir -p "$HOME/spare-test-site"
printf '<!doctype html><title>Spare test</title><h1>Spare works</h1>\n' \
  > "$HOME/spare-test-site/index.html"
spare init
spare try site "$HOME/spare-test-site"
```

On Windows PowerShell:

```powershell
$Site = Join-Path $HOME "spare-test-site"
New-Item -ItemType Directory -Force $Site | Out-Null
Set-Content (Join-Path $Site "index.html") `
  '<!doctype html><title>Spare test</title><h1>Spare works</h1>'
spare init
spare try site $Site
```

Spare prints the localhost and available LAN addresses. Check the following:

1. Open the localhost address on the Spare computer.
2. Open a LAN address from a phone or another computer on the same network.
3. Change `index.html` and refresh. The new content should appear immediately.
4. Press Ctrl-C. The temporary Site should stop.
5. Confirm that `index.html` still exists.

The operating system may ask whether to allow incoming local-network
connections. Spare does not modify firewall rules for you.

## Test the temporary lease

Start `spare try site` again, then close or forcibly terminate that terminal
without allowing normal Ctrl-C cleanup. The CLI heartbeat stops. The temporary
Site should disappear within 15 seconds.

Check its state from another terminal:

```bash
spare status
spare doctor
```

## Test a persistent Site

Install the same folder:

```bash
spare install site --path "$HOME/spare-test-site" --port auto
spare status
spare doctor
spare open dashboard
```

On Windows PowerShell, use `$Site` instead of
`"$HOME/spare-test-site"`.

The dashboard should show:

- “This computer is a Site”
- An explicit health status
- Localhost and available LAN or `.local` links
- A QR code and its destination as selectable text
- Open, Stop, or Start controls as appropriate

Test the lifecycle:

```bash
spare stop site
spare stop site
spare start site
spare start site
spare logs site
spare status --json
spare doctor --json
```

Repeating start or stop should be safe. Repeating installation with the same
path and port configuration should also be a no-op:

```bash
spare install site --path "$HOME/spare-test-site" --port auto
```

Log out and back in. The daemon should start for your user and restore the
installed Site.

## Test Drop

Remove Site before installing Drop because this preview allows one primary role:

```bash
spare remove site --yes
mkdir -p "$HOME/spare-drop"
spare install drop --path "$HOME/spare-drop" --max-file-size 10MB
spare open drop
```

From another device on the same network:

1. Open Drop using the dashboard QR code or LAN address.
2. Choose one file smaller than 10 MB.
3. Watch the browser progress indicator reach 100%.
4. Confirm that the file appears in the received-file list.
5. Download the received file and compare it with the original.
6. Upload a second file with the same name and confirm that Drop keeps both.
7. Confirm the dashboard reports received-file count and available storage.

Test the configured limit with a file larger than 10 MB. Drop should reject it
with a concrete recovery message and should not leave a partial visible file.

Inspect the destination:

```bash
ls -la "$HOME/spare-drop"
spare status
spare doctor
spare logs drop
```

Stop, start, and remove it:

```bash
spare stop drop
spare start drop
spare remove drop --yes
```

Every received file must remain in `$HOME/spare-drop`.

## Test Hook

Install Hook after removing the current role:

```bash
spare install hook
spare open hook
```

Copy the webhook endpoint shown in Hook, then send a request to it. Replace the
example URL with the one Hook displays:

```bash
curl -i -X POST \
  -H "Content-Type: application/json" \
  -H "X-Test-Signature: signed-value" \
  -d '{"event":"invoice.paid","amount":4200}' \
  "http://127.0.0.1:7340/hook/billing?source=manual"
```

Confirm that Hook shows the method, path, query, source, headers, and formatted
JSON body. In the replay form, use the same Hook origin with
`/hook/replayed` as the destination. The replay should return HTTP 202, appear
in the original request's replay history, and create a second captured request.

Also verify:

- A body larger than 1 MB returns HTTP 413 and is not stored.
- An invalid or non-HTTP replay URL shows an inline recovery message.
- The dashboard request count updates from the worker health snapshot.
- `spare stop hook` followed by `spare start hook` clears the in-memory history.
- `spare export hook` refuses to archive unrelated local files.

Remove Hook when finished:

```bash
spare remove hook --yes
```

## Test recipe packaging

```bash
spare recipe validate ./recipes/site
spare recipe validate ./recipes/drop
spare recipe validate ./recipes/hook
spare recipe pack ./recipes/site --output /tmp/site.sp
spare recipe pack ./recipes/drop --output /tmp/drop.sp
spare recipe pack ./recipes/hook --output /tmp/hook.sp
spare recipe inspect /tmp/drop.sp
spare view /tmp/drop.sp
```

The package commands should work without installing a role. A package whose ID
is not built into this release may validate, but `spare try` must refuse to
execute it.

In the package viewer, confirm:

- Recipe overview, permissions, configuration, checksum, and file sizes appear.
- Filtering the file list works with the keyboard.
- `spare.yml`, `README.md`, and `icon.svg` render as plain text.
- PNG, JPEG, GIF, and WebP package assets render as constrained images.
- Executables and unknown binary files remain listed with “Preview
  unavailable.”
- Closing the tab lets the viewer process exit after its idle window.
- Double-clicking a `.sp` package opens the viewer after installing Spare on
  macOS, Windows, and a Linux desktop with MIME tools.

## Test export and restore

Install Drop with test data, then run:

```bash
spare export drop --output /tmp/drop.spare-backup
spare remove drop --yes
spare import /tmp/drop.spare-backup --path "$HOME/spare-drop-restored" --port auto
```

Confirm that configuration and files are restored, the restored destination is
different from the original destination, and the original folder was not
removed.

## Test file-serving protections

Use test data only. The Site is visible to other reachable devices on the local
network and does not have authentication or TLS in this preview.

Verify:

- A directory with `index.html` serves that file.
- A directory without `index.html` does not show a file listing.
- A request for a dotfile such as `/.secret` is denied.
- `..` traversal attempts are denied.
- A symlink whose target stays inside the Site root works.
- A symlink whose target leaves the Site root is denied.

These cases also have unit coverage in
`internal/recipes/site/site_test.go`.

## Test removal without data loss

Add a marker file to the served folder, then remove the Site:

```bash
printf 'keep me\n' > "$HOME/spare-test-site/keep-me.txt"
spare remove site --yes
test -f "$HOME/spare-test-site/keep-me.txt"
```

On Windows PowerShell:

```powershell
Set-Content (Join-Path $Site "keep-me.txt") "keep me"
spare remove site --yes
Test-Path (Join-Path $Site "keep-me.txt")
```

The final check should succeed. Removal deletes Spare's instance metadata and
logs, not the served folder.

## Test full uninstall

Follow the uninstall instructions in [INSTALLATION.md](INSTALLATION.md), then
confirm:

- The Site folder and marker file remain.
- The Spare state directory is gone.
- The per-user login service is gone.
- The installed binaries are gone when the platform uninstaller was used.

## Platform acceptance checklist

Before publishing a tag, repeat the installed Site, Drop, and Hook tests on:

- macOS 13+ Intel
- macOS 13+ Apple Silicon
- Windows 11 amd64
- Windows 11 ARM64
- Ubuntu 22.04+
- Debian 12+
- 64-bit Raspberry Pi OS

Also test paths containing spaces and Unicode, fixed-port collisions,
unmounted folders, LAN address changes, blocked mDNS, firewall-blocked LAN
access, repeated worker crashes, and login restoration.
