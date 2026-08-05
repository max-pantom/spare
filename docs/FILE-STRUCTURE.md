# Repository structure

```text
spare/
├── .github/workflows/
│   ├── ci.yml
│   └── release.yml
├── cmd/
│   ├── spare/                  # User-facing CLI
│   ├── spare-schema/           # Versioned API schema generator
│   ├── spare-desktop/          # Wails desktop entrypoint
│   └── spared/                 # Daemon and generic worker entrypoint
├── dashboard/
│   ├── src/                    # Shared browser and desktop React surfaces
│   └── tests/                  # Playwright and axe coverage
├── desktop/
│   ├── build/darwin/           # macOS bundle metadata and helpers
│   ├── build/linux/            # Linux per-user desktop packaging
│   ├── build/windows/          # Windows per-user desktop packaging
│   ├── icons/                  # Desktop application artwork
│   └── wails.json              # Wails project metadata
├── docs/
│   ├── product-notes/          # Original supplied product documents
│   ├── schema/                 # Generated stable API schema
│   ├── API.md
│   ├── ARCHITECTURE.md
│   ├── BACKUP.md
│   ├── BUILT-IN-RECIPES.md
│   ├── DESKTOP.md
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
│   ├── apischema/              # Stable public model and endpoint schema
│   ├── artifacts/              # Packages, downloads, checksums, and cache
│   ├── auth/                   # API token and browser credentials
│   ├── backup/                 # Export and safe restore
│   ├── config/                 # Typed recipe configuration
│   ├── dashboard/              # Embedded production dashboard assets
│   ├── desktop/                # Bounded native bridge, tray, and notifications
│   ├── discovery/              # Best-effort mDNS
│   ├── doctor/                 # Diagnostic checks
│   ├── health/                 # Worker health listener and checker
│   ├── instance/               # Resolved recipe instances
│   ├── logs/                   # Rotating worker logs
│   ├── model/                  # Stable public data types
│   ├── network/                # Ports, URLs, and LAN addresses
│   ├── paths/                  # Per-user filesystem locations
│   ├── preferences/            # Shared desktop/daemon launch preferences
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
│   ├── support/                # Privacy-safe support bundle generation
│   └── supervisor/             # Lifecycle, health, leases, and restart policy
├── recipes/
│   ├── drop/                   # Distributable Drop manifest and assets
│   ├── hook/                   # Distributable Hook manifest and assets
│   └── site/                   # Distributable Site manifest and assets
├── scripts/
│   ├── release.sh
│   ├── build-desktop.sh
│   ├── build-desktop-linux.sh
│   ├── build-desktop-windows.sh
│   └── smoke.sh
├── Makefile
├── README.md
├── TODO.md
├── go.mod
└── go.sum
```

`make build` creates local binaries under ignored `bin/`. `make release`
creates checksummed archives and bundled `.sp` packages under ignored
`dist/releases/`. `make desktop-package` creates the macOS application for
the current architecture under ignored `dist/desktop/`;
`make desktop-package-amd64` and `make desktop-package-arm64` select Intel or
Apple Silicon explicitly. The Windows target uses
`make desktop-windows-package`; Linux packages are built natively with
`make desktop-linux-package`.
