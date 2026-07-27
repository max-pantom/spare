# Use the built-in recipes

Spare `0.1.0` includes three trusted recipes:

- **Site** publishes a folder as a read-only website.
- **Drop** receives files from browsers on your local network.
- **Hook** receives, inspects, and replays webhooks.

They are built into Spare, but none runs until you choose it. This preview
allows one active recipe at a time.

## Before you begin

Initialize Spare once:

```bash
spare init
```

List the recipes and their compatibility with this computer:

```bash
spare recipe list
```

Built-in recipe names are IDs, not paths. You can validate, inspect, or view
the defaults directly without finding their package files:

```bash
spare recipe validate site
spare recipe inspect drop
spare view hook
```

`validate` checks the trusted manifest and, when the bundled package is
available, confirms that it matches. `inspect` prints the manifest as JSON.
`view` locates the bundled `.sp` package and opens its safe local viewer.

The physical packages are included for inspection and distribution:

| Installation | Default package directory |
| --- | --- |
| macOS Desktop | `~/Applications/Spare.app/Contents/Resources/recipes` |
| macOS CLI | `~/Library/Application Support/Spare/recipes` |
| Windows Desktop | `%LOCALAPPDATA%\Programs\Spare\recipes` |
| Windows CLI | `%LOCALAPPDATA%\Spare\recipes` |
| Linux Desktop | `~/.local/lib/spare/recipes` |
| Linux CLI | `~/.local/state/spare/recipes` |
| Source checkout | `dist/recipes` after `make recipes` |

You do not need these paths to run a default recipe. Use its ID with commands
such as `spare try hook` or `spare install site --path ./public`.

Choose how long the recipe should run:

- `spare try` runs it temporarily. Keep the terminal open and press `Ctrl-C`
  to stop it.
- `spare install` keeps it available after the terminal closes and starts it
  again after you log in.

Automatic ports are recommended. Spare selects the first available recipe port
from `7340` through `7399`.

## Compare the recipes

| Recipe | Best for | Selected folder | Data lifetime | Network access |
| --- | --- | --- | --- | --- |
| Site | Sharing static pages, documentation, or prototypes | Required, read-only | Files remain in your folder | Other devices can read published files |
| Drop | Moving files to this computer from a browser | Required, writable | Files remain in your folder | Other devices can upload and download files |
| Hook | Inspecting and replaying webhook requests | Not used | History stays in memory until Hook stops | Other devices can submit and inspect requests; Hook can replay outbound requests |

## Site

### What Site does

Site serves one folder over HTTP. It:

- Serves `index.html` when a directory is requested.
- Reflects file changes on the next request.
- Disables directory listings.
- Denies dotfiles and path traversal.
- Allows a symlink only when its resolved target stays inside the selected
  folder.

Site is a static server. It does not provide SPA fallback, live reload,
authentication, or TLS.

### Prepare a folder

Create a folder and place an `index.html` file inside it:

```bash
mkdir -p "$HOME/spare-site"
```

For example, save this as `$HOME/spare-site/index.html`:

```html
<!doctype html>
<html lang="en">
  <meta charset="utf-8">
  <title>My Spare Site</title>
  <h1>My Spare Site is working</h1>
</html>
```

### Try Site temporarily

```bash
spare try site "$HOME/spare-site"
```

Spare prints localhost, LAN, and available `.local` addresses. Keep this
terminal open. Open a LAN address from another device connected to the same
network.

Press `Ctrl-C` to stop the temporary Site.

### Install Site

```bash
spare install site --path "$HOME/spare-site" --port auto
```

Open it later:

```bash
spare open site
```

Use a fixed port only when another tool requires one:

```bash
spare install site --path "$HOME/spare-site" --port 7350
```

If you need to change the folder or fixed port, remove Site and install it
again with the new configuration.

### Back up Site

Site exports its configuration and a copy of the selected folder:

```bash
spare export site --output site.spare-backup
```

Restore it into a new or empty folder:

```bash
spare remove site --yes
spare import site.spare-backup --path "$HOME/spare-site-restored" --port auto
```

Removing Site never deletes either selected folder.

### Use Site safely

Anyone who can reach a Site address can request its published files. Select a
folder created for public material. Do not serve credentials, private
documents, backups, or other sensitive files.

## Drop

### What Drop does

Drop provides a browser page for sending one file at a time to this computer.
It:

- Streams uploads into the selected folder.
- Shows upload progress.
- Lists received files and provides download links.
- Shows available storage.
- Defaults to a maximum of `2GB` per file.
- Preserves existing files by renaming collisions, such as
  `report (1).pdf`.
- Rejects hidden names, unsafe path characters, symlinks, and oversized files.

### Prepare a destination

Create a folder that Spare can write to:

```bash
mkdir -p "$HOME/spare-drop"
```

### Try Drop temporarily

```bash
spare try drop "$HOME/spare-drop" --max-file-size 2GB
```

Open one of the printed addresses in a browser. On a nearby device, use the LAN
address while both devices are connected to the same network.

Choose a file and select **Send file**. The received file appears in
`$HOME/spare-drop`.

Press `Ctrl-C` in the Spare terminal to stop the temporary Drop.

### Install Drop

```bash
spare install drop \
  --path "$HOME/spare-drop" \
  --max-file-size 2GB \
  --port auto
```

Open the browser interface:

```bash
spare open drop
```

You may use size values such as `500MB`, `2GB`, or `4GiB`.

To change the destination, maximum file size, or fixed port, remove Drop and
install it again with the new configuration.

### Back up Drop

Drop exports its configuration and all files currently in the selected
destination:

```bash
spare export drop --output drop.spare-backup
```

Restore into a new or empty destination:

```bash
spare remove drop --yes
spare import drop.spare-backup --path "$HOME/spare-drop-restored" --port auto
```

The original and restored folders remain untouched when the Drop instance is
removed.

### Use Drop safely

Drop has no accounts or TLS in this preview. Any device that can reach its LAN
address can upload files and download files listed by Drop. Use it only on a
network you trust, and inspect received files before opening them.

## Hook

### What Hook does

Hook is an in-memory webhook inbox. It:

- Receives any HTTP method at `/hook` and `/hook/*`.
- Shows the request method, path, query, source address, headers, and body.
- Keeps the newest 50 requests while Hook is running.
- Limits request bodies to `1MB`.
- Replays a captured method, body, and end-to-end headers to a full HTTP or
  HTTPS URL.
- Keeps the latest 20 replay attempts for each captured request.
- Records replay status, duration, response headers, and up to `64KB` of the
  response body.
- Does not follow redirects during replay.

Hook does not need a selected folder. Its request history is erased whenever
Hook stops or restarts, so it cannot be exported.

### Try Hook temporarily

```bash
spare try hook
```

Open its browser interface:

```bash
spare open hook
```

Run that command from another terminal while temporary Hook remains attached
to the first terminal. Use the actual address and port printed by Spare to send
a test request. For example:

```bash
curl -X POST \
  -H "Content-Type: application/json" \
  -d '{"order":"123"}' \
  "http://127.0.0.1:7340/hook/orders?source=test"
```

Refresh the Hook page to inspect the captured request. When sending from
another device or service on your network, replace `127.0.0.1` with the LAN
address printed by Spare.

### Replay a request

Open a captured request in the Hook page, enter a full destination beginning
with `http://` or `https://`, and select **Replay request**.

Hook preserves end-to-end headers, including authorization and cookie headers.
Confirm the destination carefully before replaying a request.

### Install Hook

```bash
spare install hook --port auto
```

Open it later:

```bash
spare open hook
```

Stopping or restarting an installed Hook clears its captured history.

### Use Hook safely

Hook has no accounts or TLS. Anyone who can reach it can submit and inspect
requests, including secrets in headers and bodies, and can initiate outbound
replays. Use Hook only on a trusted test network. Do not expose it through
router port forwarding or a public tunnel.

## Manage the active recipe

Check what Spare is doing:

```bash
spare status
spare open dashboard
```

Stop and restart an installed recipe:

```bash
spare stop site
spare start site
```

Replace `site` with `drop` or `hook` as appropriate. Start and stop commands
are safe to repeat.

Read logs and run diagnostics:

```bash
spare logs site
spare logs site --follow
spare doctor
```

Remove the active recipe:

```bash
spare remove site
```

Removal deletes Spare's instance metadata and logs. Site and Drop selected
folders, including all files inside them, remain unchanged.

To switch recipes, remove the current recipe before trying or installing the
next one:

```bash
spare remove site --yes
spare install drop --path "$HOME/spare-drop"
```

## Troubleshoot access

If a recipe does not open:

1. Run `spare status` and use the current URL and port.
2. Run `spare doctor` for daemon, folder, storage, network, and sleep checks.
3. Read its log with `spare logs <recipe>`.
4. Keep both devices on the same local network.
5. Check the operating-system firewall, VPN settings, and Wi-Fi client
   isolation.
6. Use `--port auto` if a fixed port is already occupied.

mDNS is best effort. A missing `.local` address does not make the recipe
unhealthy; use a printed LAN IPv4 address instead.
