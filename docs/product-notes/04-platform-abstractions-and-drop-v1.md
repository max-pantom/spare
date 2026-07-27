Your repo is already strong for a Site focused runtime, but it is still shaped around one worker. To make V1 feel like **Spare** instead of “a static server with a dashboard,” add the abstractions that let Site become one recipe among many.

# Add now

```text
spare/
├── cmd/
│   ├── spare/
│   ├── spared/
│   └── spare-worker/           # Optional generic worker entrypoint
│
├── internal/
│   ├── api/
│   ├── auth/
│   ├── dashboard/
│   ├── discovery/
│   ├── logs/
│   ├── model/
│   ├── paths/
│   ├── profile/
│   ├── service/
│   ├── state/
│   ├── supervisor/
│   │
│   ├── recipe/                 # .sp parsing, validation and lifecycle
│   ├── runtime/                # How recipes are executed
│   │   ├── native/
│   │   └── process/
│   ├── instance/               # Installed recipe instances
│   ├── network/                # Ports, LAN addresses, reachability
│   ├── permissions/            # Filesystem and network access rules
│   ├── health/                 # Shared recipe health checks
│   ├── config/                 # Recipe configuration values
│   ├── artifacts/              # Download, checksum and extraction
│   └── recipes/
│       ├── site/
│       └── drop/
```

## 1. `internal/recipe`

This is the most important missing layer.

It should own:

```text
Parse a .sp package
Validate the manifest
Check platform compatibility
Resolve configuration
Install a recipe
Start an instance
Stop an instance
Remove an instance
```

Core types:

```go
type Manifest struct {
    Schema      string
    ID          string
    Name        string
    Version     string
    Description string
    Runtime     RuntimeSpec
    Resources   ResourceSpec
    Network     NetworkSpec
    Storage     StorageSpec
    Health      HealthSpec
    Config      []ConfigField
}
```

Without this package, every new use case will become another hardcoded directory like `internal/site`.

## 2. `internal/runtime`

Separate recipe meaning from process execution.

```go
type Runtime interface {
    Prepare(ctx context.Context, instance Instance) error
    Start(ctx context.Context, instance Instance) (Process, error)
    Stop(ctx context.Context, instance Instance) error
    Status(ctx context.Context, instance Instance) (RuntimeStatus, error)
    Remove(ctx context.Context, instance Instance) error
}
```

Start with:

```text
native
Runs packaged binaries

process
Runs approved commands or bundled executables
```

Later add:

```text
container
managed
virtual machine
```

The supervisor should supervise runtimes, not Site directly.

## 3. `internal/instance`

A recipe is a reusable definition.

An instance is one configured installation of it.

```text
Recipe
drop

Instance
drop-primary
Storage: ~/Downloads/Spare
Port: 7340
Start on boot: true
```

Suggested model:

```go
type Instance struct {
    ID         string
    RecipeID   string
    Version    string
    Status     Status
    Runtime    string
    Port       int
    Config     map[string]any
    DataPath   string
    CreatedAt  time.Time
    StartedAt  *time.Time
}
```

This prevents recipe metadata, machine state and running process state from getting mixed together.

## 4. `internal/network`

Move general network behavior out of `site`.

It should own:

```text
Automatic port allocation
Loopback address
LAN address detection
Hostname generation
Reachability checks
mDNS names
QR code destination
Port conflicts
```

Potential API:

```go
type Endpoint struct {
    LocalURL string
    LANURL   string
    Hostname string
    Port     int
}
```

Then both Site and Drop can use the same endpoint system.

## 5. `internal/permissions`

Every `.sp` recipe should declare what it can access.

Initial permissions:

```text
Read selected folder
Write selected folder
Accept local network connections
Access the internet
Start automatically
Run in the background
```

Example manifest:

```yaml
permissions:
  filesystem:
    read:
      - selected-folder
    write:
      - selected-folder

  network:
    local: true
    internet: false
```

The installer should show:

```text
Drop can:

Receive files from your local network
Write files into Downloads/Spare

Drop cannot:

Read your other folders
Open a public internet address
```

This needs to exist before third party recipes become possible.

## 6. `internal/config`

Recipes need configurable fields without custom code in the dashboard.

Example:

```yaml
config:
  destination:
    type: directory
    label: Where should files be stored?
    required: true

  max_file_size:
    type: size
    default: 2GB

  start_on_boot:
    type: boolean
    default: true
```

The same schema should generate:

```text
CLI questions
Dashboard controls
Saved instance configuration
Validation rules
```

This removes recipe specific settings code.

## 7. `internal/artifacts`

This handles recipe binaries and packages.

Responsibilities:

```text
Download artifacts
Verify SHA256 checksums
Extract .sp packages
Select platform artifact
Cache downloads
Clean old versions
Perform atomic replacement
```

Suggested structure:

```text
internal/artifacts/
├── download.go
├── checksum.go
├── extract.go
├── platform.go
└── cache.go
```

Do not add signatures yet unless external recipe distribution is part of V1. Checksums are enough for the first internal releases.

# Refactor Site

Right now:

```text
cmd/spared
Daemon and Site worker

internal/site
Secure static-folder server
```

That couples the daemon to Site.

Change it to:

```text
cmd/spared
Spare daemon only

internal/recipes/site
Site recipe implementation
```

Or make Site a bundled recipe package:

```text
recipes/
└── site/
    ├── spare.yml
    ├── icon.svg
    └── worker/
```

The daemon should know how to run recipes, but it should not know what Site does.

# Add Drop as the public demo

```text
internal/recipes/drop/
├── server.go
├── upload.go
├── download.go
├── storage.go
├── pairing.go
└── health.go
```

V1 Drop scope:

```text
Browser upload
Browser download
QR access
Local network only
User selected folder
Transfer progress
Available storage check
Filename collision handling
Maximum file size
Temporary mode
Persistent mode
```

Avoid accounts, sync, cloud storage and remote access in V1.

# Add recipe package commands

Your CLI should gain:

```bash
spare recipe validate ./drop
spare recipe pack ./drop
spare recipe inspect drop.sp

spare try drop.sp
spare install drop.sp
spare start drop
spare stop drop
spare remove drop
```

You can later shorten:

```bash
spare pack
spare inspect
```

But namespacing early avoids command confusion.

# Add machine suitability

Your `profile` package should expose capabilities, not only raw metrics.

```go
type Capabilities struct {
    CanServeLAN        bool
    CanRunPersistent   bool
    CanStoreLargeFiles bool
    CanRunContainers   bool
    HasBattery         bool
    HasExternalStorage bool
}
```

Recipe compatibility should return:

```go
type Compatibility struct {
    Supported bool
    Rating    string
    Reasons   []string
    Warnings  []string
}
```

Example:

```text
Drop

Compatibility
Excellent

Reason
Enough memory
Local network available
42GB storage available

Warning
This laptop may sleep when the lid is closed
```

That sleep warning is especially important on laptops.

# Add `doctor`

Create:

```text
internal/doctor/
├── checks.go
├── network.go
├── storage.go
├── service.go
└── repair.go
```

Command:

```bash
spare doctor
```

Checks:

```text
Daemon running
Dashboard reachable
Recipe healthy
Port reachable
Destination writable
Storage available
Startup service valid
mDNS available
Computer sleep configuration
```

This will save you major debugging time across macOS, Windows and Linux.

# Add migration and export

The user should always be able to recover their files and configuration.

```bash
spare export drop
spare import drop.spare-backup
```

Suggested package:

```text
internal/backup/
├── export.go
├── import.go
├── manifest.go
└── archive.go
```

For V1 this can simply export:

```text
Instance configuration
Recipe version
User data
Basic metadata
```

# Dashboard additions

Your dashboard should move from a Site specific screen to four reusable views.

```text
Machine
What this computer can do

Recipes
Available and installed recipes

Instance
Status, address, storage and actions

Activity
Transfers, starts, stops, failures and updates
```

For Drop, show:

```text
Drop is ready
Open from phone
QR code
Files received
Storage remaining
Recent transfers
Stop Drop
```

Keep CPU graphs inside diagnostics, not the main screen.

# Suggested updated repository

```text
spare/
├── cmd/
│   ├── spare/
│   └── spared/
│
├── internal/
│   ├── api/
│   ├── artifacts/
│   ├── auth/
│   ├── backup/
│   ├── config/
│   ├── dashboard/
│   ├── discovery/
│   ├── doctor/
│   ├── health/
│   ├── instance/
│   ├── logs/
│   ├── model/
│   ├── network/
│   ├── paths/
│   ├── permissions/
│   ├── profile/
│   ├── recipe/
│   ├── runtime/
│   │   ├── native/
│   │   └── process/
│   ├── service/
│   ├── state/
│   ├── supervisor/
│   └── recipes/
│       ├── drop/
│       └── site/
│
├── recipes/
│   ├── drop/
│   │   ├── spare.yml
│   │   ├── icon.svg
│   │   └── README.md
│   └── site/
│       ├── spare.yml
│       ├── icon.svg
│       └── README.md
│
├── dashboard/
├── installers/
├── scripts/
├── tests/
│   ├── integration/
│   ├── recipes/
│   └── platform/
├── .github/workflows/
├── dist/releases/
├── Makefile
├── README.md
├── go.mod
└── go.sum
```

# V1 priority order

Build these in this order:

1. Refactor Site out of the daemon
2. Add Recipe and Instance models
3. Add native Runtime interface
4. Add shared network and health layers
5. Add `.sp` manifest parsing and validation
6. Build Drop
7. Add config and permission schemas
8. Add `spare try`
9. Add `spare install`
10. Add `spare doctor`
11. Add package and checksum support
12. Update the dashboard around recipes and instances

Do not add containers, a marketplace, SpareOS, remote access or generated recipes yet.

## Mana

Your current repo already has most of the hard operating system foundation. The missing step is removing **Site as an architectural concept**.

Site and Drop should both become ordinary clients of the same runtime:

```text
Site uses:
network
storage
health
supervisor
runtime

Drop uses:
network
storage
health
supervisor
runtime
```

Once this is true, the third recipe should require almost no platform work. That is the real test that Spare has become a platform rather than a collection of individually hardcoded server features.
