# Build recipe packages

Spare `0.1.0` defines a small V1 manifest and ZIP-compatible `.sp` package
format. The tooling can validate, inspect, and pack recipes. The runtime
executes only Site, Drop, and Hook implementations compiled into this release.

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
spare recipe inspect drop.sp
```

The packer sorts files, normalizes archive timestamps, and rejects symlinks and
special files so the same source can produce reproducible package content.

Build all bundled packages:

```bash
make recipes VERSION=0.1.0
```

Outputs are written to `dist/recipes`.

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

These work because their IDs resolve to trusted built-in implementations:

```bash
spare try site.sp --path ./public
spare install drop.sp --path ./received-files
spare try hook.sp
```

A valid package with another recipe ID can be validated and inspected, but
Spare refuses to run it. Third-party artifact execution needs signatures,
publisher trust, stronger permission enforcement, and isolation first.
