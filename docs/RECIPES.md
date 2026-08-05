# Build recipe packages

Spare `0.1.1-alpha.3` defines a small V1 manifest and ZIP-compatible `.sp`
package format. The tooling can validate, inspect, pack, and sign first-party
job packages. The runtime executes only trusted implementations compiled into
this release.

## Editable and distributable forms

Developers edit a directory:

```text
drop/
├── spare.yml
├── README.md
└── icon.svg
```

They distribute the packed result:

```text
drop.sp
```

The package is checksummable and extractable with standard ZIP tooling, but
people using Spare do not need to know its archive format.

## Validate a recipe

Validate a trusted built-in recipe by ID:

```bash
spare recipe validate site
```

Validate an unpacked directory:

```bash
spare recipe validate ./recipes/drop
```

Validate a manifest:

```bash
spare recipe validate ./recipes/drop/spare.yml
```

Validate a package:

```bash
spare recipe validate ./drop.sp
```

Validation rejects unknown YAML fields, unsupported schema/runtime values,
invalid IDs, missing support declarations, unsupported configuration types,
and storage fields that do not reference a declared directory input.

## Pack and inspect

Create a package:

```bash
spare recipe pack ./recipes/drop --output drop.sp
```

Inspect its normalized manifest:

```bash
spare recipe inspect drop
spare recipe inspect drop.sp
```

Open a bundled default or a package in a local browser:

```bash
spare view drop
spare view drop.sp
```

The viewer shows the validated manifest, compatibility, declared permissions,
configuration fields, package checksum, compressed and unpacked sizes, and
every packaged path. Text files are rendered as inert plain text. PNG, JPEG,
GIF, and WebP files get constrained image previews. SVG, HTML, and scripts are
shown only as text; executables, unknown formats, and files larger than 2 MB
remain listed without being opened.

The viewer listens only on a random loopback port. It stops on `Ctrl-C` or
after the browser has stopped contacting it for two minutes.

The packer sorts files, normalizes archive timestamps, and rejects symlinks and
special files so the same source can produce reproducible package content.

Build all bundled packages:

```bash
make recipes VERSION=0.1.1-alpha.3
```

Outputs are written to `dist/recipes`.

## Sign and publish an optional job

Optional first-party packages carry an Ed25519 signature envelope. Keep the
private key outside the repository:

```bash
spare recipe sign clipboard.sp \
  --key /secure/path/catalog-ed25519.pem \
  --minimum-spare-version 0.1.1-alpha.3

SPARE_CATALOG_SIGNING_KEY=/secure/path/catalog-ed25519.pem \
  make catalog VERSION=0.1.1-alpha.3
```

The catalog generator writes immutable package downloads and `catalog.json`
under `website/`. It marks a job available only after the matching trusted
implementation is part of Spare.

## V1 manifest

```yaml
schema: spare.recipe/v1
id: drop
name: Drop
version: 0.1.0
description: Send files to this computer from a browser on the local network.

support:
  systems: [darwin, windows, linux]
  architectures: [amd64, arm64]

runtime:
  type: native

resources:
  memoryRecommendedBytes: 134217728
  memoryMaximumBytes: 536870912
  cpuMaximum: 1
  storageMinimumBytes: 104857600

network:
  visibility: local
  port: automatic

storage:
  pathField: destination
  readOnly: false

health:
  type: http
  path: /
  intervalSeconds: 10
  failureThreshold: 3

config:
  destination:
    type: directory
    label: Destination folder
    required: true
  max-file-size:
    type: size
    label: Maximum file size
    default: 2GB

permissions:
  filesystem:
    read: [destination]
    write: [destination]
  network:
    local: true
    internet: false
  startOnLogin: true
  runInBackground: true
```

Supported configuration types are `string`, `directory`, `size`, `boolean`,
and `integer`.

Supported V1 runtime declarations are `native` and `process`. The process
driver accepts only a command approved and supplied by the host application;
manifest authors cannot select an arbitrary host command.

## Package execution boundary

These work because their IDs resolve to trusted implementations:

```bash
spare try site.sp --path ./public
spare install drop.sp --path ./received-files
spare try hook.sp
```

Site, Drop, and Hook are bundled. Clipboard, Downloads, and Monitor must first
be installed from a verified optional package:

```bash
spare job add ~/Downloads/clipboard_0.1.0.sp
spare install clipboard
```

Spare verifies the whole package checksum, Ed25519 signature, publisher key,
minimum compatible Spare version, exact manifest match, and downgrade rules.
It repeats verification from the private library before making an installed
job available. A valid package with an unknown ID can still be safely
inspected, but Spare refuses to install or execute it.

Optional packages never carry executable plugins. They enable the matching
first-party implementation already compiled into Spare.
