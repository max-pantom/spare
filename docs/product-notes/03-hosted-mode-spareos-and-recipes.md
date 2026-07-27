Yes. Spare should support **two ways of becoming part of a computer**, but they should share the same core.

# The two Spare modes

## 1. Spare Layer

Spare runs on top of the operating system already installed.

Your Mac remains a Mac. A Windows laptop remains a Windows laptop. Spare runs as a background service and manages recipes without taking over the whole machine.

```text
macOS, Windows or Linux
        ↓
Spare Layer
        ↓
Recipes
```

Examples:

```bash
spare try site
spare install drop
spare run webhook
```

This mode is for:

• Your daily laptop
• A work computer
• Testing recipes
• Machines that still serve another purpose
• Developers building `.sp` recipes
• Temporary services
• People trying Spare for the first time

The operating system still controls:

• User accounts
• The desktop
• Files
• Power settings
• Updates
• Hardware drivers

Spare only controls the services installed through Spare.

A good internal name is:

```text
Hosted mode
```

Because Spare is hosted by the existing operating system.

The public product language could simply say:

> Install Spare on this computer.

Users do not need to learn the name “Hosted mode” unless they open advanced settings.

## 2. SpareOS

SpareOS takes over the entire computer.

```text
Hardware
   ↓
SpareOS
   ↓
Spare Runtime
   ↓
Recipes
```

This mode is for:

• Old laptops
• Raspberry Pis
• Mini PCs
• Dedicated office machines
• Refurbished computers
• Machines that should run continuously
• Computers without monitors

The machine stops being a general desktop computer. Its main identity becomes Spare.

A good internal name is:

```text
Dedicated mode
```

The public interaction could be:

> Make this computer a dedicated Spare.

So the progression becomes:

```text
Try Spare
Run something temporarily

Install Spare
Run Spare alongside your current system

Install SpareOS
Make Spare the entire system
```

That is cleaner than treating them as two unrelated products.

# There is also a possible third mode

Later, SpareOS could be installed beside another operating system.

When the computer starts, the user chooses:

```text
Start Windows
Start SpareOS
```

This is dual booting.

Technically possible, but I would avoid it initially. It introduces disk partitioning, bootloader management, data loss risks and confusing machine state.

The first two models are enough:

```text
Existing OS plus Spare

Dedicated machine running SpareOS
```

# The `.sp` recipe format

`.sp` is a strong extension because it is short, directly connected to Spare and feels like an object the user can pass around.

Examples:

```text
drop.sp
site.sp
agent-scratchbox.sp
pantom-review.sp
```

The user could install one directly:

```bash
spare install drop.sp
```

Try it without installing:

```bash
spare try drop.sp
```

Inspect it:

```bash
spare inspect drop.sp
```

Validate one being developed:

```bash
spare validate pantom-review.sp
```

Package a project:

```bash
spare pack ./pantom-review
```

## What should a `.sp` file actually be?

There are two options.

### Plain manifest

The `.sp` file is readable configuration text.

```yaml
spare: 1

id: drop
name: Drop
version: 0.1.0

about:
  description: Transfer files between nearby devices.
  category: sharing

support:
  systems:
    - macos
    - windows
    - linux
    - spareos

  architectures:
    - amd64
    - arm64

runtime:
  type: native
  command: drop

network:
  visibility: local
  port: automatic

storage:
  directories:
    files:
      location: user-selected
      required: true

health:
  type: http
  path: /health
  interval: 30s

permissions:
  network:
    local: true
    internet: false

  files:
    read:
      - files
    write:
      - files

resources:
  memory:
    recommended: 512MB
    maximum: 1GB

  cpu:
    maximum: 1

interface:
  open: /
```

This is easy to write and understand.

But it cannot contain the recipe’s executable, icons or supporting files.

### Packaged recipe

The `.sp` file is an archive containing everything needed.

```text
drop.sp
├── recipe.yml
├── README.md
├── icon.svg
├── checksums.json
├── signature.sig
├── assets/
└── binaries/
    ├── darwin-arm64/
    │   └── drop
    ├── darwin-amd64/
    │   └── drop
    ├── windows-amd64/
    │   └── drop.exe
    ├── linux-amd64/
    │   └── drop
    └── linux-arm64/
        └── drop
```

This is the better long term model.

The `.sp` file can internally be a ZIP compatible archive, but users never need to know that.

Developers work with an unpacked recipe folder:

```text
drop/
├── spare.yml
├── icon.svg
├── README.md
└── src/
```

Then package it:

```bash
spare pack ./drop
```

Output:

```text
Created drop.sp
Platforms: 5
Size: 18.4MB
Signature: valid
```

So:

```text
spare.yml
The editable recipe definition

.sp
The distributable recipe package
```

That distinction is important.

# A `.sp` file should support multiple runtimes

A recipe should be able to describe different ways of running depending on the current computer.

```yaml
runtime:
  preference:
    - native
    - container
    - managed

  native:
    artifacts:
      darwin-arm64: binaries/darwin-arm64/drop
      darwin-amd64: binaries/darwin-amd64/drop
      windows-amd64: binaries/windows-amd64/drop.exe
      linux-amd64: binaries/linux-amd64/drop
      linux-arm64: binaries/linux-arm64/drop

  container:
    image: ghcr.io/spare-run/drop@sha256:abc123

  managed:
    engine: node
    version: "24"
    entrypoint: app/server.js
```

Spare selects the best option.

On your Mac:

```text
Selected runtime
Native macOS ARM64
```

On SpareOS:

```text
Selected runtime
Container
```

On a platform where no native binary exists:

```text
Native version unavailable.
Spare can run this recipe inside its Linux container environment.
```

# Containers should absolutely be part of Spare

But they should be an internal capability, not the product definition.

The user should think:

```text
I installed a recipe.
```

Not:

```text
I created a container and configured volumes, ports and networks.
```

## Why containers matter

Containers give Spare:

• Isolation between recipes
• Repeatable execution
• Easier third party packaging
• Controlled storage mounts
• Resource limits
• Cleaner upgrades
• Easier rollback
• Linux application compatibility
• Fewer dependency conflicts

They become especially valuable on SpareOS because Spare controls the complete environment.

## Containers in Hosted mode

### Linux

Containers can run directly through a container engine such as containerd or Podman.

```text
Linux
  ↓
Spare
  ↓
Container runtime
  ↓
Recipe container
```

### macOS and Windows

Most server containers are Linux containers. A Linux virtual machine is therefore required underneath them on macOS and Windows.

```text
macOS or Windows
        ↓
Spare
        ↓
Small managed Linux VM
        ↓
Container runtime
        ↓
Recipe container
```

Existing container products already use this broad architecture. Spare should hide and simplify it.

The user could run:

```bash
spare containers enable
```

Spare responds:

```text
Container support requires a small Linux environment.

Reserved resources
Memory: 2GB
Storage: 12GB
CPU: 2 cores

This environment only runs when a container recipe is active.
```

Then Spare handles:

• Creating the VM
• Starting it when needed
• Shutting it down
• Port forwarding
• Mounting approved directories
• Updating the environment
• Recovering it after failure

## Containers in SpareOS

SpareOS should have container support built in.

No separate VM is required because SpareOS itself is Linux based.

```text
SpareOS
   ↓
containerd or Podman
   ↓
Recipe containers
```

This is one major reason SpareOS will eventually be the best environment for serious Spare use.

# Native recipes and container recipes should both remain first class

Do not make every recipe a container.

A static site server, file drop or webhook inbox can be a tiny native binary. Requiring a Linux VM on a Mac just to serve a folder would be wasteful.

Use this rule:

```text
Native when the service is small and official.

Managed runtime when the service is written in Node or Python.

Container when the service has complex dependencies.

VM when stronger isolation or another operating system is required.
```

A recipe’s actual runtime should be visible under technical details, but it should not dominate the normal interface.

```text
Drop

Status
Healthy

Runtime
Native

Memory
38MB
```

Or:

```text
Immich

Status
Healthy

Runtime
Container environment

Memory
1.4GB
```

# Go, Rust or Zig

For Spare, raw execution speed is not the main question.

The main requirements are:

1. Reliable long running daemon
2. Strong networking
3. macOS, Windows and Linux support
4. ARM64 and x64 builds
5. Process management
6. Filesystem operations
7. HTTP API
8. Concurrency
9. Small deployment footprint
10. Easy maintenance
11. Fast development
12. Eventually deep operating system integration

All three languages can produce fast native binaries. The difference is where each one makes the project easier or harder.

# Go

## Strengths

Go is extremely well suited to the first Spare runtime.

It gives you:

• Simple cross platform binaries
• Fast compilation
• Excellent networking libraries
• Straightforward concurrency through goroutines
• A strong standard library
• Easy HTTP servers
• Good CLI tooling
• Low operational complexity
• Mature support for Windows, macOS, Linux and ARM64
• A garbage collector that is acceptable for a daemon like Spare

Go compiles to native machine code and provides built in tooling for building and testing packages. Its target operating system and architecture model makes producing binaries for multiple supported platforms straightforward, especially while avoiding unnecessary C dependencies. ([Go][1])

A release build can be conceptually simple:

```bash
GOOS=darwin GOARCH=arm64 go build
GOOS=windows GOARCH=amd64 go build
GOOS=linux GOARCH=amd64 go build
GOOS=linux GOARCH=arm64 go build
```

## Weaknesses

• Garbage collection
• Larger binaries than carefully optimized Rust or Zig binaries
• Less precise memory control
• Foreign system APIs sometimes require platform specific packages or C bindings
• Not ideal for writing kernels, bootloaders or very low level OS components

For `spare`, `spared`, networking, recipe management and the dashboard API, those weaknesses are not major.

## Best use in Spare

```text
CLI
Daemon
Recipe engine
Local API
Networking
Discovery
Update client
Health monitoring
Most official recipe binaries
```

# Rust

## Strengths

Rust provides:

• Memory safety without garbage collection
• Excellent performance
• Strong type system
• Precise resource control
• Good concurrency safety
• Strong suitability for security sensitive components
• Better foundations for low level system work
• Good cross compilation support across many defined compiler targets

Rust’s compiler is designed as a cross compiler, and its target system supports producing binaries for a wide range of platforms and architectures. ([Rust Documentation][2])

Rust becomes attractive for:

• Container isolation components
• Privileged helpers
• Sandboxing
• Filesystem mounting
• Low level networking
• SpareOS system services
• Security critical parsers
• Boot or recovery tooling

## Weaknesses

• Slower development for a small team
• Longer compile times
• More complex ownership and lifetime model
• Async Rust can become complicated
• Cross compiling applications with native dependencies can still require platform toolchains
• More engineering effort for ordinary service code

Rust would likely make the initial Spare daemon safer at the memory level, but it would also make the early product slower to build.

## Best use in Spare

Rust is a strong alternative if your team is already highly effective in Rust.

Otherwise, it is best introduced for focused low level components later.

```text
Security sensitive helpers
Container supervisor
Sandbox boundary
SpareOS updater
Filesystem tooling
Privileged operations
```

# Zig

## Strengths

Zig gives you:

• Direct memory control
• No garbage collector
• Small native binaries
• First class cross compilation
• Strong C interoperability
• Simple deployment
• Good control over allocators
• Potentially excellent performance
• A useful build system for native projects

Cross compilation is one of Zig’s explicit first class use cases, and Zig includes strong support for targeting other operating systems and architectures without requiring conventional external cross toolchains for many cases. ([Zig Programming Language][3])

Zig is especially interesting for:

• Tiny recipe binaries
• Boot utilities
• Recovery tools
• Cross compiled native helpers
• Replacing small C components
• Building C dependencies across platforms

## Weaknesses

• Smaller ecosystem
• Fewer mature libraries
• More manual memory management
• Less mature application level web and service tooling
• Smaller hiring pool
• Greater maintenance risk for a large cross platform product
• Language and ecosystem stability are not at the same level as Go or Rust

Zig is technically exciting, but Spare’s early difficulty will come from operating system integration and product behavior, not CPU performance.

Using Zig for the entire initial platform would add avoidable product risk.

## Best use in Spare

```text
Small utilities
Boot and recovery components
Performance critical native helpers
Cross compiling C dependencies
Tiny official recipes
```

# My recommendation

Use **Go for the main Spare runtime**.

```text
spare CLI           Go
spared daemon       Go
Local API           Go
Recipe engine       Go
Profiler            Go
Health system       Go
Networking          Go
Updater              Go initially
Dashboard           TypeScript and React
```

Then introduce Rust where a component has stronger security or system level requirements.

```text
Privileged helper       Rust
Sandbox controller      Rust
SpareOS updater         Rust
Mount helper            Rust
Container boundary      Rust
```

Use Zig selectively for very small portable utilities or as a cross compilation tool where it provides a clear advantage.

```text
Boot helper             Zig or Rust
Tiny recovery command   Zig
C build tooling         Zig
Small recipe binaries   Zig, Go or Rust
```

The practical architecture is therefore not “choose one language forever.”

It is:

```text
Go for the control plane.

Rust for hard security boundaries.

Zig for tiny low level tools where its simplicity matters.
```

# Is Go fast enough?

Yes.

Spare is not primarily performing large mathematical calculations. It is mostly:

• Waiting for events
• Reading system information
• Starting processes
• Managing configuration
• Serving local HTTP requests
• Monitoring health
• Reading logs
• Moving metadata
• Coordinating recipes

These are I/O heavy tasks.

The difference between Go, Rust and Zig will rarely be visible to the user in the main daemon.

A well designed Go daemon might consume somewhat more memory than an aggressively optimized Rust or Zig daemon, but it can still remain small enough for Raspberry Pi and old computer use.

A reasonable initial target would be:

```text
Spare daemon idle memory
Below 50MB

CLI startup
Below 100ms where practical

Dashboard API response
Below 50ms locally

Installer binary
Below 30MB compressed

Idle CPU
Near 0%
```

Architecture and dependencies will matter more than the language.

A badly designed Rust daemon can use more resources than a carefully designed Go daemon.

# Recommended technical split

```text
spare
Command line client

spared
Persistent cross platform daemon

spare-ui
Shared dashboard

sparevm
Managed Linux environment for container recipes on Mac and Windows

spare-agent
Small component running inside the managed VM

spareos
Dedicated operating system distribution
```

## Hosted installation

```text
macOS, Windows or Linux
           ↓
         spared
           ↓
   Native recipe runtime
           ↓
 Optional sparevm for containers
```

## Dedicated installation

```text
SpareOS
   ↓
spared
   ↓
Native and container runtimes
```

Both use:

```text
Same .sp packages
Same CLI
Same API
Same dashboard
Same state model
Same recipe registry
```

# Updated product language

> **Spare gives any computer a job.**

Run it alongside macOS, Windows or Linux, or install SpareOS and turn the entire machine into dedicated infrastructure.

A more technical description:

> Spare is a cross platform runtime and dedicated operating system for packaging, running and managing useful services as `.sp` recipes.

# Mana

The two modes should not be designed around whether a computer is old or new.

They should be designed around **ownership of the machine**.

```text
Spare Layer
Spare borrows part of the computer.

SpareOS
Spare owns the computer.
```

That distinction determines everything:

```text
Who controls updates
Who controls power
Who controls storage
Who controls networking
Who guarantees availability
Who can run containers efficiently
```

On your MacBook, Spare must behave like a respectful tenant. It uses limited resources and stops when requested.

On a dedicated Raspberry Pi or old laptop, Spare becomes the landlord. It can manage the whole machine for reliability, isolation, automatic recovery and container workloads.

That gives you one product capable of moving from a five minute local experiment to a permanent piece of infrastructure without changing the recipe itself.

[1]: https://go.dev/doc/install/source?utm_source=chatgpt.com "Installing Go from source - The Go Programming Language"
[2]: https://doc.rust-lang.org/rustc/targets/index.html?highlight=target&utm_source=chatgpt.com "Targets - The rustc book"
[3]: https://ziglang.org/learn/overview/?utm_source=chatgpt.com "Overview ⚡ Zig Programming Language"
