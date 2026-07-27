# Security boundary

Spare `0.1.0` is intended for trusted local computers and networks. The
dashboard is private to the current computer. Recipe web interfaces are
deliberately available to reachable devices on the local network.

## Management API

- The dashboard and `/api/v1` listen only on `127.0.0.1`.
- CLI calls use a randomly generated 256-bit bearer token.
- The token and state directory are restricted to the current user.
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

V1 runs only Site, Drop, and Hook implementations compiled into the daemon. A
package cannot introduce a new executable command. External recipe execution
remains blocked until artifact signatures, publisher trust, isolation, and
permission enforcement are designed.

## Backups

`spare export` copies the entire selected folder into the explicitly requested
backup archive. A backup may therefore contain sensitive content.

Recipes without selected-folder data, including Hook, cannot be exported.

Restore rejects traversal and symlinks and requires an empty destination so it
cannot overwrite existing user files. Backup archives are not encrypted.
Protect them like the selected folder.

## Process supervision

Recipes run as child workers of the unprivileged per-user daemon. Each has a
separate loopback health endpoint. Repeated failures trigger controlled
restarts, and the crash-rate limit stops a repeatedly failing worker.

This preview does not provide a container, VM, or operating-system sandbox
around built-in workers.

## Release security

Release archives and `.sp` packages include SHA-256 checksums. The preview does
not yet include code signing, macOS notarization, automatic updates, or a
signed update channel. Verify checksums and use builds from a trusted source.

Report a suspected vulnerability privately to the repository maintainers. Do
not include API tokens, private paths, backup archives, or served files.
