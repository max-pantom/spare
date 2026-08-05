# Security boundary

Spare `0.1.1-alpha.3` is intended for trusted local computers and networks. The
dashboard is private to the current computer. Recipe web interfaces are
deliberately available to reachable devices on the local network.

## Management API

- The dashboard and `/api/v1` listen only on `127.0.0.1`.
- CLI calls use a randomly generated 256-bit bearer token.
- The token is strictly parsed as a 256-bit value and repaired to owner-only
  permissions when Spare starts.
- The token, endpoint record, logs, packages, and job state are restricted to
  the current user. Existing directories are repaired to private permissions.
- The endpoint record is size-bounded, rejects unknown data and symlinks, and
  can identify only Spare's reserved `127.0.0.1` ports. This prevents a changed
  endpoint file from forwarding the bearer token to another host.
- `spare open dashboard` creates a short-lived, single-use browser code.
- The browser exchanges that code for an HttpOnly, SameSite cookie.
- The long-lived token is never placed in a browser URL.
- CORS is disabled and mutation requests with invalid origins are rejected.

Do not copy the API token into bug reports or commit it to source control.

## LAN exposure

Site, Drop, and Hook listen on localhost and non-loopback IPv4 interfaces. They
do not provide authentication or TLS in this preview.

- Anyone who can reach Site can request its public files.
- Anyone who can reach Drop can upload files into its selected destination and
  download the received files listed there.
- Anyone who can reach Hook can submit and inspect requests, including secret
  values, and initiate outbound replays.

Use these recipes only on a network you trust. Do not expose them through
router port forwarding, a public tunnel, or a public cloud firewall rule.

Clipboard, Downloads, and Monitor use a separate trusted-device gate. A nearby
device enters a locally displayed six-digit code, receives a random 24-hour
HttpOnly and SameSite cookie, and can be revoked from the job interface.
Repeated wrong codes are rate-limited, and each session is bound to the source
IP address that completed pairing. Session state and the number of active
sessions are bounded. Cookies use the Secure flag when the job is served with
TLS; the normal local HTTP interface cannot provide that transport guarantee,
so these jobs must not be exposed beyond a trusted LAN.

Spare does not:

- Open or modify host firewall rules
- Configure a router
- Create public internet exposure
- Provide remote access
- Add recipe accounts or authentication

## Site protections

Site:

- Canonicalizes the selected root
- Denies traversal and dotfile segments
- Disables directory listings
- Serves `index.html` for directories
- Allows symlinks only when their resolved target remains inside the root
- Opens content read-only

Do not serve secrets, private documents, personal backups, or regulated data.

## Drop protections

Drop:

- Requires an existing writable destination selected by the current user
- Streams one multipart file at a time instead of buffering it in memory
- Enforces a configurable per-file size limit
- Checks available destination storage
- Writes through a temporary file followed by an atomic rename
- Rejects hidden or unsafe filenames
- Resolves collisions without overwriting existing files
- Lists and downloads only regular non-symlink files in the destination root
- Applies restrictive browser security headers

Drop is not malware scanning, content moderation, access control, or a durable
backup system. Review received files before opening them.

## Clipboard protections

Clipboard:

- Limits individual text and file entries, total text storage, total file
  storage, and the number of active entries
- Receives one upload at a time, bounds request bodies, preserves a free-space
  reserve, and removes multipart temporary files
- Uses random private filenames and confines reads, writes, and deletion to the
  owner-only Clipboard file root
- Sanitizes displayed filenames and forces downloads to use an attachment
  disposition
- Removes expired entries and their files

Clipboard content is not encrypted at rest and is available to every currently
paired device until it expires or is deleted. Treat paired devices as mutually
trusted and review transferred files before opening them.

## Downloads protections

Downloads:

- Accepts only HTTP and HTTPS URLs without embedded credentials
- Resolves and dials destinations itself, without an environment proxy
- Rejects loopback, private, link-local, carrier-grade NAT, documentation, and
  other special-use addresses before connecting
- Rechecks every redirect and rejects HTTPS-to-HTTP downgrades
- Limits queue length and file size, preserves a destination storage reserve,
  and stops stalled transfers
- Uses response validators for safe resume and rejects mismatched ranges
- Confines partial and completed files to the selected folder, rejects
  symlinks, and never overwrites a colliding file
- Keeps complete source URLs in owner-only job state so interrupted transfers
  can resume, but does not include those URLs in displayed errors

Signed download URLs may contain credentials in their query string. They remain
in the private local job state until the item is removed; this state is not
encrypted at rest.

## Monitor protections

Monitor:

- Limits the number of targets and the size and history of persisted state
- Uses direct bounded HTTP connections instead of environment proxy settings
- Rejects credential-bearing redirects and HTTPS-to-HTTP downgrades
- Bounds redirects, response headers, connection stages, and total check time
- Does not persist complete HTTP errors or display URL query values
- Validates HTTP, TCP, and ping targets and passes ping hosts as a single
  process argument

Monitoring local devices is an intentional feature, so a paired device can ask
this computer to make HTTP, TCP, or ping probes to private network addresses.
Use Monitor only with devices you trust. Full target URLs remain in owner-only
local state so checks can continue after restart; that state is not encrypted
at rest.

## Hook protections

Hook:

- Keeps a bounded history of 50 requests and 20 replay attempts per request in
  worker memory
- Rejects bodies larger than 1 MB
- Renders captured values as text instead of executable HTML
- Accepts replay destinations only as full HTTP or HTTPS URLs without embedded
  credentials or fragments
- Rejects replay mutations from a different browser origin
- Removes hop-by-hop headers and recalculates the destination host and body
  length
- Does not follow redirects while replaying
- Applies restrictive browser security headers

Hook intentionally preserves end-to-end headers, including authorization and
cookie values, because exact inspection and replay are its purpose. Its local
network interface is not access control. Captured history disappears whenever
the Hook worker stops or restarts.

## Recipe packages

The `.sp` tooling:

- Uses known-field manifest validation
- Accepts only the V1 native or approved-process schema
- Creates reproducible ZIP-compatible packages
- Rejects traversal, absolute paths, special files, and symlinks during package
  extraction
- Provides SHA-256 calculation and verification
- Selects artifacts by explicit OS and architecture keys
- Verifies first-party Ed25519 signatures and exact trusted manifest matches
- Rejects altered, unsigned, unknown, incompatible, and downgraded optional
  packages
- Allows only bounded setup metadata in optional catalog packages; downloaded
  executables and unexpected payload files are rejected
- Copies a candidate into private staging storage and reverifies the installed
  copy before recording it
- Revalidates installed package files before exposing their implementations

The `.sp` browser viewer:

- Binds only to a random `127.0.0.1` port
- Revalidates the manifest and every archive path before opening
- Rejects traversal, symlinks, special files, duplicate paths, and
  case-conflicting paths
- Limits previews to 2 MB
- Serves SVG, HTML, scripts, manifests, and documentation as inert plain text
- Previews only PNG, JPEG, GIF, and WebP as images
- Never previews packaged executables or unknown binary formats
- Uses a restrictive content security policy and disables framing and objects
- Stops on `Ctrl-C` or after two minutes without browser activity

V1 runs only implementations compiled into the daemon. Site, Drop, and Hook
are bundled; Clipboard, Downloads, and Monitor are enabled by verified
first-party metadata packages. A package cannot introduce a new executable
command. Third-party recipe execution remains blocked.

Each optional job receives a private owner-only state directory. Package
installation, review, switching, and removal are local bearer-only operations.
Uninstalling a package removes its registration but deliberately preserves
job state, downloaded content, and user-selected folders.

## Backups

`spare export` copies the entire selected folder into the explicitly requested
backup archive. A backup may therefore contain sensitive content.

Recipes without selected-folder data, including Hook, cannot be exported.

Restore rejects traversal and symlinks and requires an empty destination so it
cannot overwrite existing user files. Backup archives are not encrypted.
Pairing codes are omitted and regenerated after restore. Protect backups like
the selected folder.

## Process supervision

Recipes run as child workers of the unprivileged per-user daemon. Each has a
separate loopback health endpoint. Repeated failures trigger controlled
restarts, and the crash-rate limit stops a repeatedly failing worker.

The macOS login agent creates private files by default. The Linux user service
also removes ambient capabilities, blocks privilege gain, isolates temporary
files, and enables available systemd process and kernel hardening. The Windows
scheduled task explicitly uses the current user's limited run level.

Run `spare doctor --security` to inspect local state permissions, the control
endpoint, login-service hardening, the executable checksum, installed package
signatures, LAN exposure, and the worker-isolation boundary. The command also
supports `--json` for support and CI tooling; review the output before sharing
it because it contains local paths and addresses.

`spare support bundle` is the safer artifact for a support request. It includes
coarse platform, job-state, package-signature, and diagnostic statuses, while
excluding API tokens, hostnames, machine IDs, addresses, URLs, configuration,
paths, logs, activity contents, backups, private job data, and files from
user-selected folders.

This preview does not provide a container, VM, or operating-system sandbox
around built-in workers.

## Release security

Release archives and `.sp` packages include SHA-256 checksums. Optional `.sp`
packages also carry first-party Ed25519 signatures. GitHub release builds emit
artifact provenance attestations. Pushes and pull requests run pinned CodeQL,
race detection, Go vulnerability analysis, and npm auditing workflows; pull
requests also run dependency review. Provenance identifies the source workflow
and commit; it does not make an unsafe artifact safe and is not platform code
signing.

The preview does not yet include application code signing, macOS notarization,
automatic updates, or a signed application update channel. Verify checksums
and provenance, and use builds from a trusted source.

Report a suspected vulnerability privately to the repository maintainers. Do
not include API tokens, private paths, backup archives, or served files.
