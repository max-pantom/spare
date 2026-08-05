# Install and use optional jobs

Spare `0.1.1-alpha.3` introduces a first-party job catalog. Site, Drop, and
Hook stay bundled. Clipboard, Downloads, and Monitor are optional downloads.
Archive, Media, DNS, Ad Blocker, and Cameras are catalog roadmap entries and
are not installable yet.

## Install from Spare Desktop

1. Open **Jobs** and select **Install more jobs**.
2. Download a `.sp` package from the Spare catalog.
3. Open or drag the package into Spare.
4. Review its publisher, version, checksum, signature, storage access, and
   network access.
5. Select **Install job**.

Installation does not stop or replace the active job. Select the installed job
later, configure it, and confirm the switch. Spare saves the previous job's
settings. If the new job cannot start, Spare restores the previous one.

An optional package can be uninstalled only while another job is active.
Uninstall removes the package registration but never deletes downloaded files,
temporary job state, or user-selected folders.

## Install from the CLI

```bash
spare job add ~/Downloads/clipboard_0.1.0.sp
spare install clipboard
spare job remove clipboard
```

The CLI prints the verified permissions before installation. `spare view
package.sp` remains the safe, inert package inspector.

## Clipboard

Clipboard moves text, links, and small files between paired devices. Entries
use a selected expiration time and are cleaned from owner-only private state.
There is no permanent history by default.

## Downloads

Downloads accepts ordinary HTTP and HTTPS file links from paired devices. It
runs one transfer at a time and supports a visible queue, progress, speed,
pause, range-based resume, retry, cancel, and opening completed files.
Downloads stay inside the destination selected on the Spare computer.

## Monitor

Monitor checks manually added HTTP addresses, hosts, and TCP ports. It records
recent response times, last success, and current state. Spare emits
deduplicated local failure and recovery activity, which Desktop can deliver as
notifications.

## Pair a nearby device

After activating an optional LAN job, open its local address from the trusted
device and enter the six-digit code shown in the job's configuration. The
session expires after 24 hours. Use **Revoke devices** to invalidate every
paired session immediately.

Pairing protects changes to optional job interfaces, but the local connection
normally uses HTTP rather than TLS. Keep Spare on a trusted network and do not
forward its ports or expose them through a public tunnel.
