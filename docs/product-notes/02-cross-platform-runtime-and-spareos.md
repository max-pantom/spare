Exactly. Spare should not be defined around old computers.

That is only one strong use case.

The broader idea is:

> **Spare turns any computer into useful infrastructure.**

That computer could be:

• Your current MacBook
• A Windows desktop
• An old laptop
• A Raspberry Pi
• A mini PC
• A VPS
• A machine dedicated entirely to Spare
• Eventually, custom Spare hardware

The user should be able to install Spare on the computer they already have, test a recipe, stop it, remove it and continue using the computer normally.

Then, when they have dedicated hardware, they can install SpareOS and turn the whole machine over to Spare.

# The product should have two forms

## 1. Spare Runtime

This is the main product.

It installs on an existing operating system:

```bash
curl -fsSL https://spare.run/install | sh
```

Or:

```bash
brew install spare
```

```powershell
irm https://spare.run/install.ps1 | iex
```

```bash
npm install -g @spare/cli
```

After installation:

```bash
spare init
```

Spare profiles the current computer and starts a local management service.

The computer still remains a Mac, Windows computer or Linux computer. Spare simply gives it the ability to run and manage services.

This is what you should build first.

## 2. SpareOS

SpareOS is the dedicated version.

It is installed when someone wants the entire computer to become a Spare machine.

This could be:

• A Raspberry Pi
• An old laptop with no useful operating system
• A mini PC
• A refurbished desktop
• A dedicated office server
• A headless machine in another room

SpareOS would include:

• The Spare runtime
• The dashboard
• Networking
• Storage management
• Updates
• Drivers
• Recovery tools
• Recipe execution
• Remote and local management

The critical distinction is:

```text
Spare Runtime
Runs inside your current computer

SpareOS
Makes Spare the computer
```

Both should use the same recipes, commands, API and dashboard.

# Do not build SpareOS first

Building an operating system first would make the wrong thing the bottleneck.

You would immediately inherit:

• Hardware drivers
• WiFi compatibility
• Boot failures
• Disk partitioning
• Secure Boot
• Firmware differences
• GPU support
• Sleep and power management
• Laptop keyboards and trackpads
• Installers
• Recovery partitions
• Operating system updates

You would spend most of your time making computers boot instead of proving Spare.

SpareOS should initially be a prepared Linux image built from an existing distribution, not an operating system written from scratch.

Underneath, it could use Debian, Ubuntu Server or another stable Linux base. The user does not need to know that.

```text
SpareOS
├── Linux kernel
├── Hardware drivers
├── Spare daemon
├── Recipe runtime
├── Local dashboard
├── Update system
└── Recovery environment
```

SpareOS is a product layer over Linux, similar to how many purpose built systems use Linux internally while presenting their own focused interface.

# The correct technical architecture

Spare should have five main layers.

```text
┌─────────────────────────────────────────────┐
│              Spare Interfaces               │
│                                             │
│     CLI        Dashboard        Desktop      │
├─────────────────────────────────────────────┤
│               Spare Daemon                  │
│                                             │
│ Profiles, installs, supervises, monitors    │
├─────────────────────────────────────────────┤
│              Recipe Engine                  │
│                                             │
│ Native, packaged and isolated workloads     │
├─────────────────────────────────────────────┤
│          Operating System Adapter           │
│                                             │
│ macOS       Windows       Linux              │
├─────────────────────────────────────────────┤
│                 Hardware                    │
│                                             │
│ x64       ARM64       Storage       Network  │
└─────────────────────────────────────────────┘
```

## Layer 1: Spare CLI

The CLI is the first interface.

```bash
spare init
spare profile
spare recipes
spare install drop
spare start drop
spare stop drop
spare open
spare status
spare doctor
spare logs
spare remove drop
```

You should be able to install Spare on your Mac now and run:

```bash
spare install site
```

Then:

```bash
spare site ./my-prototype
```

And Spare could respond:

```text
Your site is running.

Local
http://localhost:7340

Nearby devices
http://max-macbook.local:7340

Dashboard
http://localhost:7331
```

You can test the product entirely on your main laptop without dedicating the machine.

## Layer 2: Spare daemon

The persistent background process could be called:

```text
spared
```

It should be a single compiled executable.

I recommend Go because it gives you:

• Cross platform binaries
• Good networking support
• Easy concurrency
• Low memory usage
• ARM64 and x64 compilation
• Straightforward daemon development
• No runtime dependency required on the user’s machine

The daemon would expose a local API:

```text
http://127.0.0.1:7331/api
```

Both the CLI and dashboard communicate with this API.

```text
spare CLI ───────┐
                 ├──── Local API ──── spared
Spare Dashboard ─┘
```

The daemon owns:

• Machine profiling
• Recipe management
• Process supervision
• Port allocation
• Health checks
• Local discovery
• Logs
• Permissions
• Updates
• Storage locations
• Backups
• Authentication
• Runtime adapters

## Layer 3: Operating system adapters

Spare should have one unified internal interface with separate implementations for every operating system.

Conceptually:

```go
type Platform interface {
    InstallService(config ServiceConfig) error
    StartService(name string) error
    StopService(name string) error
    RestartService(name string) error
    ServiceStatus(name string) (Status, error)
    ReadSystemMetrics() (Metrics, error)
    OpenFirewallPort(port int) error
    DiscoverStorage() ([]Disk, error)
}
```

Then:

```text
platform/
├── darwin/
├── windows/
└── linux/
```

### macOS

Spare should register its background daemon through `launchd`, the native macOS service manager. Apple uses `launchd` to start, supervise and restart agents and daemons. ([Apple Support][1])

Conceptually:

```text
~/Library/LaunchAgents/run.spare.daemon.plist
```

Or, where system level privileges are required:

```text
/Library/LaunchDaemons/run.spare.daemon.plist
```

For the first version, you should prefer a per user service where possible. This reduces permission prompts and makes testing safer.

### Windows

Spare should install `spared.exe` as a Windows Service. Windows Services are designed for long running background processes that can start automatically at boot and continue without an interactive user session. ([Microsoft Learn][2])

Conceptually:

```text
Spare Service
Binary: C:\Program Files\Spare\spared.exe
Startup: Automatic
```

You may also include:

```text
spare.exe
```

for the CLI and:

```text
Spare Desktop.exe
```

for a tray application later.

### Linux

Linux would use systemd.

Systemd manages services, restarts processes, tracks resources through control groups and supports dependency based service management. ([Systemd][3])

Conceptually:

```text
/etc/systemd/system/spared.service
```

The important point is that users should never need to interact with any of these service managers directly.

They only run:

```bash
spare start
```

# The hardest decision: how recipes run

Running services across macOS, Windows and Linux creates a major technical problem.

A Linux container does not run natively on macOS or Windows. Tools such as Podman solve this by creating a Linux virtual machine beneath the container runtime. ([docs.podman.io][4])

Therefore Spare should not make containers mandatory for every recipe from day one.

You need a mixed runtime architecture.

## Runtime 1: Native

The recipe downloads the correct executable for the machine.

```yaml
runtime:
  type: native

artifacts:
  darwin-arm64:
    url: ...
  darwin-amd64:
    url: ...
  windows-amd64:
    url: ...
  windows-arm64:
    url: ...
  linux-amd64:
    url: ...
  linux-arm64:
    url: ...
```

Spare then runs the binary using the native operating system service manager.

This is the best runtime for the first recipes.

Advantages:

• Fast startup
• Low memory usage
• No virtual machine
• Works on current laptops
• Works on Raspberry Pi
• Small downloads
• Easier local testing

Disadvantage:

Every recipe needs binaries compiled for supported systems and architectures.

## Runtime 2: Script

For simple development recipes:

```yaml
runtime:
  type: process
  executable: node
  command:
    - server.js
```

Or:

```yaml
runtime:
  type: process
  executable: python
  command:
    - -m
    - webhook_inbox
```

This is useful during development but should not be the ideal user experience because it depends on Node or Python being installed.

Spare could eventually manage these runtimes itself.

```yaml
runtime:
  type: managed
  engine: node
  version: "24"
  entrypoint: server.js
```

Spare downloads an isolated Node runtime rather than using the user’s installation.

## Runtime 3: Container

Containers should be supported where they make sense.

```yaml
runtime:
  type: container
  image: ghcr.io/spare-run/drop@sha256:...
```

On Linux and SpareOS, this can run directly.

On macOS and Windows, Spare would need to install or manage a lightweight Linux virtual machine. Podman similarly requires a virtual machine on both platforms because the containers themselves are Linux based. ([docs.podman.io][4])

Containers should therefore be:

• Optional during the early stage
• Invisible to normal users
• Primarily used for complex third party recipes
• Native on SpareOS and Linux
• Backed by a managed VM on macOS and Windows

## Runtime 4: Virtual machine

Later, Spare could support full VM recipes.

```yaml
runtime:
  type: vm
  image: spare://ubuntu-agent-runner
```

This could be useful for:

• Unsafe agent workloads
• Linux only applications on Mac and Windows
• Stronger isolation
• Development sandboxes
• Operating system testing

But this is not necessary for the first release.

# The correct first runtime strategy

Start with:

```text
Official recipes
Native executables

Developer recipes
Managed Node or native processes

Complex recipes
Containers later

Untrusted agent work
VMs much later
```

Do not require Docker, Podman or virtualization just to demonstrate Spare.

# The recipe system needs platform awareness

A recipe should be capable of supporting several execution methods.

```yaml
schema: spare.recipe/v1

id: drop
name: Drop
description: Transfer files between nearby devices.

support:
  systems:
    - macos
    - windows
    - linux

  architectures:
    - amd64
    - arm64

runtime:
  preferred: native

  native:
    darwin-arm64:
      artifact: drop-darwin-arm64
    darwin-amd64:
      artifact: drop-darwin-amd64
    windows-amd64:
      artifact: drop-windows-amd64.exe
    linux-amd64:
      artifact: drop-linux-amd64
    linux-arm64:
      artifact: drop-linux-arm64

  fallback:
    type: container
    image: ghcr.io/spare-run/drop:1.0.0

network:
  port: automatic
  visibility: local

data:
  directory: user-selected

health:
  type: http
  endpoint: /health
```

Spare selects the best runtime for the current machine.

```text
MacBook with Apple Silicon
Selected: native darwin-arm64

Windows PC
Selected: native windows-amd64

Raspberry Pi
Selected: native linux-arm64

SpareOS mini PC
Selected: container linux-amd64
```

# Supporting Raspberry Pi from the beginning

You do not necessarily need a Raspberry Pi on the first day, but the architecture should support it.

The main requirement is ARM64 compilation.

Your release matrix should include:

```text
macOS
darwin-arm64
darwin-amd64

Windows
windows-amd64
windows-arm64

Linux
linux-amd64
linux-arm64
```

For Raspberry Pi, target 64 bit Raspberry Pi OS first.

Installation:

```bash
curl -fsSL https://spare.run/install | sh
```

The installer detects:

```text
Operating system: Linux
Distribution: Raspberry Pi OS
Architecture: ARM64
Service manager: systemd
```

Then downloads the correct Spare binary.

Spare should also understand machine character.

```text
Machine type
Raspberry Pi 5

Suitable for
File transfer
Static sites
Webhook inbox
Network tools
Light agent tasks

Limited for
Large AI models
Video transcoding
High traffic services
```

# Spare should be testable without changing the computer

You need two execution modes.

## Temporary mode

```bash
spare try drop
```

This runs the recipe temporarily without installing a persistent service.

```text
Drop is running temporarily.

It will stop when this terminal closes.

Local address
http://localhost:7340

Nearby devices
http://max-macbook.local:7340
```

This is essential for onboarding.

Someone can experience Spare before allowing it to run in the background.

## Installed mode

```bash
spare install drop
```

This registers the recipe with Spare and keeps it running.

```text
Drop has been installed.

It will restart automatically and can run after reboot.
```

## Disposable test mode

```bash
spare try ./spare.yml
```

Developers can test a recipe locally.

```bash
spare validate ./spare.yml
spare try ./spare.yml
spare package ./spare.yml
```

This also makes Spare useful as a development tool before it becomes a machine management platform.

# Installation experience

## macOS

```bash
brew install spare
```

Or:

```bash
curl -fsSL https://spare.run/install | sh
```

Then:

```bash
spare init
```

Expected output:

```text
Welcome to Spare.

Machine
Max’s MacBook Pro

System
macOS 26
Apple Silicon
16GB memory
184GB available

Spare can run temporarily without changing startup settings.

Choose a mode:

1. Try Spare
2. Install background service
3. Open dashboard
```

## Windows

```powershell
winget install Spare.Spare
```

Or:

```powershell
irm https://spare.run/install.ps1 | iex
```

Then:

```powershell
spare init
```

The installer should be signed before public distribution. Unsigned Windows executables and macOS binaries will produce security warnings that can destroy the “simple package” experience.

## Linux and Raspberry Pi

```bash
curl -fsSL https://spare.run/install | sh
```

Or later:

```bash
apt install spare
```

# What the GUI should be

You do not need to choose between CLI and GUI.

The CLI and GUI are two clients connected to the same Spare daemon.

```text
                  ┌──────────────────┐
                  │     Spare CLI    │
                  └────────┬─────────┘
                           │
┌──────────────────┐       │       ┌──────────────────┐
│ Browser Dashboard├───────┼───────┤ Desktop Wrapper  │
└──────────────────┘       │       └──────────────────┘
                           │
                    ┌──────▼──────┐
                    │   spared    │
                    └─────────────┘
```

The dashboard should be served locally by the daemon:

```text
http://spare.local
```

Or on the machine:

```text
http://localhost:7331
```

This means the exact same dashboard can work on:

• macOS
• Windows
• Linux
• Raspberry Pi
• SpareOS
• Headless machines

You do not need separate dashboard applications for each platform.

Later, you can wrap the dashboard in a lightweight desktop shell for:

• Menu bar access on macOS
• System tray access on Windows
• Notifications
• Native file selection
• Starting and stopping Spare
• Easy onboarding

But the web dashboard remains the canonical interface.

# How SpareOS should behave

SpareOS should have two interface modes.

## Headless mode

The machine has no monitor.

After installing SpareOS, it displays or announces:

```text
Spare is ready.

Open:
http://spare.local

Pairing code:
482 193
```

The user manages it from their phone or laptop.

This should be the default for Raspberry Pi and mini PCs.

## Connected mode

When the machine has a screen, SpareOS displays a minimal local interface.

Not a desktop.

Not folders, a browser, settings applications and taskbars.

The whole screen is Spare.

```text
┌─────────────────────────────────────────────┐
│ SPARE                                       │
│                                             │
│ This machine is ready                       │
│                                             │
│ No role installed                           │
│                                             │
│ [ Choose a role ]                           │
│                                             │
│ Open from another device                    │
│ spare.local                                 │
│                                             │
│ Network        Connected                    │
│ Storage        462GB available              │
│ System         Healthy                      │
└─────────────────────────────────────────────┘
```

Once a recipe is installed:

```text
┌─────────────────────────────────────────────┐
│ SPARE                                       │
│                                             │
│ This machine is a Drop                      │
│                                             │
│ Ready                                       │
│                                             │
│ spare.local                                 │
│                                             │
│ Files received                 28            │
│ Storage available              441GB         │
│ Uptime                         16 days        │
│                                             │
│ [ Open ] [ Manage ] [ Change role ]         │
└─────────────────────────────────────────────┘
```

The local SpareOS UI should be a kiosk interface powered by the same dashboard application.

# A possible SpareOS architecture

```text
SpareOS

Linux kernel
    ↓
Minimal Debian or Ubuntu base
    ↓
Systemd
    ↓
Spare runtime
    ↓
Recipe runtime
    ↓
Spare dashboard
    ↓
Kiosk shell or remote browser
```

Later, you may move to an immutable system design.

An immutable system keeps the base operating system mostly read only. Updates replace the system image rather than modifying hundreds of packages individually.

That would give SpareOS:

• Predictable machines
• Easier recovery
• Atomic updates
• Less configuration drift
• Factory reset
• Safer remote upgrades

But the first SpareOS prototype can be much simpler.

# The central data model

Spare should not model everything as an application.

It should model four things:

## Machine

```json
{
  "id": "spare_max_macbook",
  "hostname": "max-macbook",
  "system": "darwin",
  "architecture": "arm64",
  "mode": "hosted",
  "status": "ready"
}
```

## Capability

```json
{
  "cpu": {
    "cores": 8
  },
  "memory": {
    "totalBytes": 17179869184
  },
  "storage": {
    "availableBytes": 197568495616
  },
  "features": {
    "virtualization": true,
    "battery": true,
    "gpu": true
  }
}
```

## Recipe

```json
{
  "id": "drop",
  "version": "0.1.0",
  "runtime": "native",
  "requirements": {
    "memoryBytes": 268435456
  }
}
```

## Instance

```json
{
  "id": "drop_primary",
  "recipeId": "drop",
  "status": "healthy",
  "port": 7340,
  "dataDirectory": "/Users/max/Spare/drop",
  "startedAt": "2026-07-26T15:12:00Z"
}
```

This model allows the same recipe to run on your current laptop, an old computer, a Raspberry Pi or SpareOS.

# Repository structure

A serious initial monorepo could look like:

```text
spare/
├── cmd/
│   ├── spare/
│   └── spared/
│
├── internal/
│   ├── api/
│   ├── daemon/
│   ├── machine/
│   ├── profiler/
│   ├── recipes/
│   ├── runtime/
│   │   ├── native/
│   │   ├── process/
│   │   └── container/
│   ├── supervisor/
│   ├── networking/
│   ├── storage/
│   ├── health/
│   └── platform/
│       ├── darwin/
│       ├── windows/
│       └── linux/
│
├── dashboard/
│   ├── app/
│   ├── components/
│   └── lib/
│
├── recipes/
│   ├── drop/
│   ├── site/
│   └── hook/
│
├── installers/
│   ├── install.sh
│   ├── install.ps1
│   ├── homebrew/
│   └── winget/
│
├── packaging/
│   ├── macos/
│   ├── windows/
│   ├── linux/
│   └── spareos/
│
└── tests/
    ├── integration/
    ├── platform/
    └── recipes/
```

# What I would build as the first real vertical slice

Not the full profiler.

Not the recipe catalog.

Not SpareOS.

Build one path that works completely:

```text
Install Spare
↓
Initialize the machine
↓
Run a temporary recipe
↓
Open it from another device
↓
Install it persistently
↓
See its health
↓
Stop and remove it cleanly
```

The first commands:

```bash
spare init
spare try site ./public
spare install site --path ./public
spare status
spare open
spare stop site
spare remove site
```

The first recipe should probably be **Site**, not Drop.

Site is technically simpler because it only needs to serve a directory. It proves:

• Installation
• Architecture detection
• Process supervision
• Local ports
• LAN access
• Dashboard communication
• Health checks
• Persistent services
• Cross platform packaging

Then build Drop once the runtime works.

# Suggested build order

## Milestone 1: Cross platform shell

Build:

• Go CLI
• Go daemon
• Local API
• SQLite state
• macOS service adapter
• Windows service adapter
• Linux systemd adapter
• Machine and architecture detection
• Local dashboard shell

Supported targets:

```text
darwin-arm64
darwin-amd64
windows-amd64
linux-amd64
linux-arm64
```

## Milestone 2: Temporary recipe execution

Build:

```bash
spare try site ./folder
```

Include:

• Port allocation
• Native process runner
• Local hostname
• QR code
• Health endpoint
• Logs
• Clean shutdown

## Milestone 3: Persistent installation

Build:

```bash
spare install site
```

Include:

• Start at boot
• Restart after failure
• Stored configuration
• Dashboard state
• Stop, start and remove

## Milestone 4: Real recipe package format

Build:

• Recipe manifest
• Artifact downloads
• Checksums
• Platform selection
• Version management
• Compatibility validation
• Signed official recipes

## Milestone 5: Drop

Build the first strong consumer utility:

• File upload
• File download
• QR pairing
• Storage folder selection
• Basic authentication
• Device history
• Expiry options

## Milestone 6: SpareOS prototype

Build:

• Bootable Linux image
• Automatic Spare startup
• First boot pairing
• Kiosk dashboard
• Network setup
• Factory reset
• USB installer

# Revised product definition

> Spare turns any computer into useful infrastructure.

Install it on the laptop you already use, a Raspberry Pi, an old desktop or a dedicated machine. Run a service temporarily, keep it running in the background or install SpareOS and make it the computer’s entire purpose.

The strongest first line remains:

> **Give any computer a job.**

Not:

> Turn old computers into servers.

The old computer story becomes one demonstration:

```text
Your current laptop can test it.
Your Raspberry Pi can run it.
Your old computer can become it.
```

# Mana

Spare Runtime and SpareOS should not be treated as two different products.

They are two levels of commitment.

```text
spare try
I want to experience this job temporarily

spare install
I want this computer to keep doing this job

SpareOS
I want this job to become the computer
```

That progression is the product architecture.

Someone can begin on their MacBook today without needing another device. When the recipe becomes valuable enough, Spare can tell them:

> This service has run for 47 hours on your laptop. Move it to a Raspberry Pi or dedicated Spare machine so it can keep running when your laptop is closed.

That creates a natural path from experimentation to permanent infrastructure without forcing the user to understand deployment, migration or server administration.

[1]: https://support.apple.com/en-lamr/guide/terminal/apdc6c1077b-5d5d-4d35-9c19-60f2397b2369/mac?utm_source=chatgpt.com "Script management with launchd in Terminal on Mac - Apple Support"
[2]: https://learn.microsoft.com/en-us/windows/win32/services/about-services?utm_source=chatgpt.com "About Services - Win32 apps | Microsoft Learn"
[3]: https://systemd.io/?utm_source=chatgpt.com "System and Service Manager"
[4]: https://docs.podman.io/en/stable/markdown/podman-machine-start.1.html?utm_source=chatgpt.com "podman-machine-start — Podman documentation"
