# Repository structure

```text
spare/
├── .github/workflows/
│   ├── ci.yml
│   └── release.yml
├── cmd/
│   ├── spare/                  # User-facing CLI
│   └── spared/                 # Daemon and generic worker entrypoint
├── dashboard/
│   ├── src/                    # React/TypeScript status interface
│   └── tests/                  # Playwright and axe coverage
├── docs/
│   ├── product-notes/          # Original supplied product documents
│   ├── API.md
│   ├── ARCHITECTURE.md
│   ├── BACKUP.md
│   ├── BUILT-IN-RECIPES.md
│   ├── FILE-STRUCTURE.md
│   ├── INSTALLATION.md
│   ├── RECIPES.md
│   ├── SECURITY.md
│   ├── TESTING.md
│   └── USAGE.md
├── installers/
│   ├── install.ps1
│   ├── install.sh
│   ├── uninstall.ps1
│   └── uninstall.sh
├── internal/
│   ├── api/                    # Authenticated loopback API and client
│   ├── artifacts/              # Packages, downloads, checksums, and cache
│   ├── auth/                   # API token and browser credentials
│   ├── backup/                 # Export and safe restore
│   ├── config/                 # Typed recipe configuration
│   ├── dashboard/              # Embedded production dashboard assets
│   ├── discovery/              # Best-effort mDNS
│   ├── doctor/                 # Diagnostic checks
│   ├── health/                 # Worker health listener and checker
│   ├── instance/               # Resolved recipe instances
│   ├── logs/                   # Rotating worker logs
│   ├── model/                  # Stable public data types
│   ├── network/                # Ports, URLs, and LAN addresses
│   ├── paths/                  # Per-user filesystem locations
│   ├── permissions/            # Declared recipe permissions
│   ├── profile/                # Machine metrics and capabilities
│   ├── recipe/                 # Manifest and `.sp` package lifecycle
│   ├── recipeview/             # Safe loopback `.sp` package browser
│   ├── recipes/
│   │   ├── drop/               # Browser file receiver
│   │   ├── hook/               # Local webhook inbox
│   │   └── site/               # Read-only static server
│   ├── runtime/
│   │   ├── native/             # Trusted built-in worker driver
│   │   └── process/            # Host-approved process driver
│   ├── service/                # Login service registration
│   ├── state/                  # SQLite migrations and persistence
│   └── supervisor/             # Lifecycle, health, leases, and restart policy
├── recipes/
│   ├── drop/                   # Distributable Drop manifest and assets
│   ├── hook/                   # Distributable Hook manifest and assets
│   └── site/                   # Distributable Site manifest and assets
├── scripts/
│   ├── release.sh
│   └── smoke.sh
├── Makefile
├── README.md
├── TODO.md
├── go.mod
└── go.sum
```

`make build` creates local binaries under ignored `bin/`. `make release`
creates checksummed archives and bundled `.sp` packages under ignored
`dist/releases/`.
