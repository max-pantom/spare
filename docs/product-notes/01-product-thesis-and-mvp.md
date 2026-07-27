



# Spare

**Spare turns an unused computer into one dependable local utility.**

The important word is not server. It is **utility**.

“Turn your computer into a server” describes the implementation. It does not describe why someone should care.

The user wants:

> “Make this old laptop receive files from my phone.”

> “Give my agent somewhere safe to run temporary work.”

> “Let my team test webhooks without paying for another service.”

> “Turn this external drive into a searchable media shelf.”

Spare should convert those requests into a machine that quietly performs one job.

## 1. The real product thesis

CasaOS, Umbrel, YunoHost and similar products simplify self hosting by giving people operating systems, dashboards and app catalogs. Umbrel explicitly positions installation like installing an app on a phone, while YunoHost describes itself as an operating system for simplifying server administration. citeturn743778search8turn743778search5

Spare should make a more aggressive simplification:

**The machine is not the product. The job is the product.**

Existing systems generally follow this model:

```text
Install server system
Explore applications
Choose several applications
Configure networking
Manage the server
```

Spare should follow this:

```text
Install Spare
Tell Spare what the machine should do
Spare evaluates the hardware
Spare installs the smallest suitable recipe
The machine becomes that thing
```

That creates a meaningful product boundary.

Spare is not:

1. A personal cloud operating system
2. A Docker dashboard
3. A homelab control panel
4. A replacement for a cloud provider
5. A remote access network
6. An app store containing hundreds of unrelated services

Spare is:

> A local appliance generator for ordinary computers.

## 2. The strongest opening market

Do not initially market Spare to homelab users.

They already know Docker, Proxmox, Tailscale, TrueNAS and Linux. They will judge Spare by how many advanced settings it exposes.

The more valuable initial users are:

1. Developers with an unused laptop
2. Designers and small studios sharing large files locally
3. AI builders needing temporary agent machines
4. Schools and small offices with unused desktops
5. Families with old computers and external drives
6. Small teams that need one internal tool without another monthly subscription

The first user should probably be a developer or technical creative because they tolerate an early CLI while still experiencing the problem.

The opening message could be:

> You already own a server. It is sitting unused.

That is significantly stronger than talking about self hosting.

## 3. Spare should have three product modes

### Mode 1: Convert

Spare is installed on an existing Linux computer.

```bash
curl -fsSL spare.run/install | sh
spare init
```

This is the MVP.

It should support Ubuntu and Debian first. Supporting every Linux distribution, macOS and Windows immediately would turn hardware compatibility into the main project.

### Mode 2: Rescue

A user creates a bootable Spare USB.

They plug it into an old computer and choose:

```text
Try Spare without changing this computer
Install Spare alongside the current system
Erase the machine and make it a Spare
```

This becomes important later because many old computers contain broken or cluttered operating systems.

### Mode 3: Adopt

Spare discovers another computer on the local network and guides the user through preparing it.

```bash
spare adopt 192.168.1.42
```

This would require SSH or a temporary pairing agent, but it creates the closest experience to “turn any computer into a server.”

The long term product is probably all three. The MVP should only implement Convert.

## 4. The machine profile is one of Spare’s core inventions

`spare init` should not simply collect CPU and RAM.

It should produce a **capability profile**.

```yaml
machine:
  architecture: x86_64
  cpu:
    cores: 4
    generation: unknown
    virtualization: true
  memory:
    total: 8GB
    available: 6.2GB
  storage:
    system:
      available: 118GB
      health: good
    external:
      available: 1.8TB
      removable: true
  network:
    ethernet: true
    wifi: true
    local_address: 192.168.1.32
  power:
    battery_present: true
    battery_health: weak
  capabilities:
    sustained_service: suitable
    storage_service: suitable
    local_ai: limited
    video_transcoding: unsuitable
```

Spare then turns this into a human explanation:

```text
This machine is suitable for:

Excellent
File transfer
Webhook inbox
Static sites
Document archive

Good
Media indexing
Agent scratchbox
Local backups

Not recommended
Large language models
Video transcoding
Important databases
```

That recommendation engine is more important than a large recipe catalog.

The user should never install something that the machine cannot run comfortably.

## 5. Recipes should describe outcomes, not software

Umbrel has a broad application store containing many categories and hundreds of self hosted tools. That proves users value convenient installation, but Spare should avoid becoming another catalog of product names. citeturn743778search1turn743778search18

A Spare recipe is not just a Docker Compose file.

It should define:

1. What job it performs
2. What resources it requires
3. What data it owns
4. Which ports it needs
5. How it is checked
6. How it is backed up
7. How it is upgraded
8. How it is completely removed
9. What happens when the machine is under pressure
10. Whether it may be exposed outside the local network

Example:

```yaml
schema: spare.recipe/v1

name: drop
title: Local File Drop
description: Send files between devices on the same network.

requirements:
  memory:
    minimum: 256MB
    recommended: 512MB
  storage:
    minimum: 1GB
  architectures:
    - amd64
    - arm64

runtime:
  type: container
  image: ghcr.io/spare-run/drop:1.0.0
  port: 7340
  restart: always
  memory_limit: 512MB
  cpu_limit: 1

data:
  mounts:
    - name: files
      path: /data
      user_selectable: true

network:
  visibility: local
  authentication: pairing_code

health:
  endpoint: /health
  interval: 30s
  failure_threshold: 3

backup:
  paths:
    - /data
    - /config

updates:
  channel: stable
  rollback: true

actions:
  open: http://{{hostname}}.local:7340
  clear_received_files: internal://clear
```

This makes a recipe an operational contract, not merely an installation script.

## 6. “One useful service” needs a precise interpretation

Your current idea says one useful local service at a time.

That is a good constraint, but it should mean:

> One primary role per machine.

A file transfer recipe may internally need:

1. The actual application
2. A local reverse proxy
3. A small Spare health agent
4. A discovery service
5. A backup process

Those are implementation components, not separate user facing services.

The dashboard should still say:

```text
This computer is a File Drop
```

Not:

```text
5 containers running
```

This is where Spare separates itself from infrastructure tools. It hides the internal topology and preserves one understandable identity.

Later, advanced users could unlock multiple roles, but the default should remain one role.

## 7. The Spare runtime

Spare needs a small persistent daemon.

```text
spared
```

Its responsibilities should include:

1. Hardware profiling
2. Recipe installation
3. Service supervision
4. Health checks
5. Resource enforcement
6. Local discovery
7. Logs
8. Updates
9. Backup and restoration
10. Dashboard API
11. Pairing and permissions

A possible architecture:

```text
┌──────────────────────────────────────────────┐
│                  User                        │
│                                              │
│      CLI             Local Dashboard         │
└──────────────┬───────────────┬───────────────┘
               │               │
               ▼               ▼
┌──────────────────────────────────────────────┐
│                 spared                       │
│                                              │
│  Profiler       Recipe Engine                │
│  Supervisor     Health Engine                │
│  Storage        Networking                   │
│  Updates        Backup                       │
│  Events         Local API                    │
└───────────────────┬──────────────────────────┘
                    │
                    ▼
┌──────────────────────────────────────────────┐
│              Recipe Runtime                  │
│                                              │
│   Native process or rootless container       │
└──────────────────────────────────────────────┘
```

## 8. Container technology should remain invisible

Your “no Docker first ceremony” principle is right.

That does not necessarily mean Spare should avoid containers internally.

It means the user should never need to understand them.

Podman can run rootless containers and its Quadlet system can translate declarative container files into ordinary systemd services. This gives Spare service startup, restart behavior and integration with Linux service management without requiring the user to operate Docker directly. citeturn320079search1

Systemd portable services are another potential backend. They package related services into transferable filesystem images and can run them with sandboxing. citeturn320079search0

My recommended MVP stack is:

```text
Go daemon
SQLite state database
systemd service supervision
Rootless Podman for isolated recipes
Quadlet generated by Spare
Local web dashboard
mDNS discovery
```

Do not build a custom container runtime.

Do not make Docker a required public concept.

Do not let each recipe execute arbitrary shell scripts as root.

## 9. Spare needs a trust model

The moment Spare can install recipes, it becomes a software distribution system.

That is the serious part of the product.

Every official recipe should have:

1. Signed manifests
2. Pinned image digests
3. Declared filesystem access
4. Declared network access
5. Memory and CPU limits
6. Reproducible builds where possible
7. Vulnerability scanning
8. Review history
9. Automatic rollback
10. A visible trust level

Example:

```text
Official
Reviewed and maintained by Spare

Verified
Built by an external publisher and reviewed by Spare

Community
Published by the community. Extra permissions require confirmation

Local
Created on this machine
```

The dashboard should explain permissions in ordinary language:

```text
This recipe can:

Read and write the folder you selected
Accept connections from your local network
Use up to 512MB of memory

This recipe cannot:

Read your other files
Open public internet ports
Access other devices automatically
Run as the root user
```

The difference between a useful product and a dangerous script runner will be this trust system.

## 10. Local networking should feel automatic

After installation, the user should see:

```text
Your File Drop is available at:

http://spare.local
http://192.168.1.32

Nearby devices can open this address while connected to the same network.
```

Spare should provide:

1. Automatic local hostname
2. Port conflict resolution
3. QR code for phones
4. Device pairing
5. Optional local HTTPS
6. Clear local network status
7. No router configuration by default

Tailscale should be an optional adapter, not part of the product’s identity. Tailscale Serve can securely expose a local service inside a user’s private tailnet, while Funnel can expose a service publicly. citeturn743778search9turn743778search17

Spare could eventually support:

```bash
spare access local
spare access private
spare access public
```

Where:

```text
local
Only the current WiFi or Ethernet network

private
Selected authenticated devices through Tailscale

public
Explicit temporary public exposure through Wormkey or another provider
```

Public exposure must always be deliberate and reversible.

## 11. The dashboard should be a control surface, not an analytics product

The main screen should answer five questions:

1. What is this computer doing?
2. Is it working?
3. Where can I open it?
4. Is its data safe?
5. Does anything need attention?

Example:

```text
Spare

This computer is a File Drop

Ready
Available at spare.local

Storage
42GB free

Received
18 files this week

Running
4 days, 7 hours

Next action
Connect a backup drive
```

Below that:

```text
Open File Drop
Send address to phone
View files
Back up
Stop
Change role
```

Avoid CPU charts unless something is wrong.

Instead of:

```text
CPU 91%
RAM 87%
Load average 4.3
```

Say:

```text
This machine is slowing down.

File indexing is using most of its memory.
Spare has temporarily reduced indexing speed.
```

The dashboard should interpret infrastructure rather than merely display it.

## 12. The first recipes should form a coherent system

Your current recipes are good, but they cover several different audiences.

I would organize the first release around three recipe families.

### Share

#### Drop

Receive and send files across the local network.

#### Shelf

Index one folder or external drive and make it searchable.

#### Site

Serve a folder as a local website.

### Build

#### Hook

A local webhook inbox with request history and replay.

#### Preview

Serve prototypes with automatic refresh.

#### Cache

A local package, artifact or model cache.

### Agent

#### Scratchbox

A temporary workspace for agent tasks with storage and resource limits.

#### Runner

Run scheduled scripts or agent jobs.

#### Inbox

Accept tasks through a simple local API and queue them.

The best launch recipe is probably **Drop**.

It is immediately understandable, visually demonstrable and useful without explaining self hosting.

The best strategic recipe is probably **Scratchbox**, because it connects Spare to the growing need for inexpensive AI execution infrastructure.

## 13. Agent Scratchbox could become the bigger company

Scratchbox should not initially mean “run an autonomous AI agent on random old hardware.”

It should mean:

> Give an agent a controlled place to perform temporary work.

Example:

```bash
spare install scratchbox
```

The user receives:

```text
Endpoint
http://spare.local:7412

Workspace
/data/scratchbox

Limits
2 CPU cores
2GB memory
20GB storage

Allowed actions
Run code
Create files
Download approved resources

Blocked actions
Access personal folders
Control the host
Open public ports
Run indefinitely
```

Later, a coding agent could ask Spare:

```json
{
  "task": "render these 200 thumbnails",
  "requirements": {
    "memory": "2GB",
    "storage": "5GB",
    "timeout": "30m"
  }
}
```

Spare would decide whether the machine is capable, run the task and return the output.

At that point Spare becomes:

> A local compute substrate for agents.

That is much larger than a home server package.

## 14. Reliability must be part of the interface

Old computers have weak batteries, failing disks, poor cooling and unreliable network connections.

Spare should not hide that.

It should build a **confidence score** for each role.

```text
Role confidence: Good

This machine is suitable for temporary and replaceable work.

Not recommended for:
Your only copy of important files
Critical databases
Public production services
Jobs that must run continuously
```

Spare should track:

1. Disk health
2. Unexpected shutdowns
3. Memory pressure
4. Thermal pressure
5. Network changes
6. Battery state
7. Recipe crashes
8. Backup recency
9. Available storage
10. Update failures

Systemd supports watchdog based service supervision, where missing health notifications can cause an unresponsive service to be terminated and restarted. citeturn320079search7

The important difference is that Spare should translate this into actions:

```text
The service stopped responding at 14:32.
Spare restarted it automatically.
No files were lost.
```

## 15. The state model should be simple

Each Spare node can exist in one of these states:

```text
Uninitialized
Profiling
Ready
Installing
Running
Attention
Recovering
Stopped
Retired
```

Each recipe can exist in:

```text
Available
Compatible
Not recommended
Installed
Starting
Healthy
Degraded
Failed
Updating
Rolling back
Removed
```

Do not expose internal container states unless the user enters developer mode.

## 16. The CLI needs a smaller vocabulary

Your current CLI is close, but `serve` may be confusing because the service should persist automatically after installation.

I would propose:

```bash
spare init
spare find
spare use drop
spare status
spare open
spare stop
spare start
spare change
spare logs
spare backup
spare doctor
spare remove
```

The core workflow becomes:

```bash
spare init
spare find
spare use drop
spare open
```

Example output:

```text
$ spare find

This machine is best suited for:

1. Drop
   Transfer files between nearby devices

2. Hook
   Receive and inspect webhooks

3. Site
   Serve a prototype folder

Not recommended:

Local AI
This machine has insufficient available memory
```

`spare doctor` should explain and repair problems.

```text
$ spare doctor

Checking File Drop...

Service              healthy
Storage              healthy
Local address         reachable
Backup                not configured
Updates               current

One recommendation:

Connect another drive to create automatic backups.
```

## 17. The recipe recommendation engine is the moat

The recipe store itself will not be defensible. Others can copy manifests.

The deeper asset is the system that understands:

```text
This machine
This workload
This environment
This level of reliability
This user’s intent
```

And makes a safe decision.

A recipe should declare not just minimum requirements, but workload curves:

```yaml
performance:
  idle_memory: 120MB
  memory_per_active_user: 40MB
  storage_growth:
    base: 100MB
    per_1000_items: 80MB
  cpu_pattern: burst
  disk_pattern: sequential
  network_pattern: local_burst
```

Spare can estimate:

```text
This computer can comfortably support:

5 simultaneous users
Approximately 180,000 indexed files
Around 6 agent jobs per hour

These are estimates based on the current machine profile.
```

Over time, anonymized runtime information could improve the recommendations.

That creates a compatibility intelligence layer across old hardware.

## 18. A possible `spare.yml`

Spare should also let developers package their own services.

```yaml
name: pantom-review
version: 0.1.0
description: Local review board for design files

source:
  type: git
  repository: https://github.com/example/pantom-review
  revision: 9a31f04

build:
  runtime: node
  command: npm run build

run:
  command: npm start
  port: 3000

resources:
  memory: 512MB
  cpu: 1
  storage: 2GB

data:
  directories:
    - ./uploads

health:
  path: /api/health

access:
  default: local

backup:
  directories:
    - ./uploads
    - ./database
```

Then:

```bash
spare use ./spare.yml
```

This creates a bridge between simple recipes and developer deployment.

Spare eventually becomes a lightweight deployment target:

```bash
spare deploy pantom-review
```

But the word “deploy” should not dominate the consumer interface.

## 19. The update system is critical

A recipe update should work transactionally:

```text
Download new version
Verify signature
Check available storage
Create data snapshot
Start new version
Run health checks
Switch traffic
Keep old version temporarily
Roll back if unhealthy
```

The user sees:

```text
File Drop was updated successfully.

Previous version kept for 24 hours in case recovery is needed.
```

Updates should have three channels:

```text
Stable
Delayed, tested updates

Current
Normal release schedule

Manual
Nothing changes without approval
```

For old machines, stable should be the default.

## 20. Business model

Do not begin by charging for recipes.

The recipes are necessary to make the platform useful. Charging for every local job will make the product feel like a worse cloud subscription.

A stronger model is:

### Free

1. One Spare node
2. Official basic recipes
3. Local dashboard
4. Manual backups
5. Community support

### Spare Plus

Approximately $5 to $10 per month.

1. Multiple nodes
2. Remote health monitoring
3. Private remote access setup
4. Encrypted offsite configuration backup
5. Notifications
6. Automatic recovery reports
7. Advanced recipes

### Spare Teams

1. Shared node inventory
2. Role based access
3. Fleet policies
4. Approved recipe catalogs
5. Audit logs
6. Remote updates
7. Agent workload scheduling

### Spare Certified

A future hardware program for refurbished computers.

```text
Spare Certified
Tested for local services
Securely erased
Hardware health checked
Ready for three years of updates
```

Spare could partner with refurbishers rather than manufacture hardware.

## 21. What I would build first

### Milestone 1: One machine, one recipe

Build:

1. Ubuntu and Debian support
2. `spared` daemon
3. `spare init`
4. CPU, memory, disk and network profile
5. Local SQLite state
6. Systemd installation
7. One native or rootless container recipe
8. `spare use drop`
9. `spare status`
10. Local dashboard
11. `.local` network discovery
12. QR code access
13. Basic health checks
14. Automatic restart
15. Complete uninstall

Success condition:

> A person can take an unused laptop and turn it into a working local file drop in under ten minutes without installing Docker manually.

### Milestone 2: Trust and recovery

Build:

1. Signed recipes
2. Resource limits
3. Permissions display
4. Backups
5. Updates
6. Rollbacks
7. `spare doctor`
8. Disk health checks
9. Crash explanations
10. Recipe compatibility scoring

### Milestone 3: Developer recipes

Build:

1. `spare.yml`
2. Local recipe packaging
3. Hook recipe
4. Site recipe
5. Preview recipe
6. Logs
7. Secrets
8. Data export
9. Local registry cache

### Milestone 4: Agent infrastructure

Build:

1. Scratchbox
2. Task API
3. Execution limits
4. Workspace isolation
5. Job history
6. Artifact return
7. Agent authentication
8. Machine capability matching

### Milestone 5: Spare network

Build:

1. Multiple node dashboard
2. Private remote access
3. Node pairing
4. Workload placement
5. Replication
6. Fleet updates
7. Remote recovery

## 22. The sharper product statement

Your current statement:

> Spare is a small package that prepares an unused computer to run one useful local service at a time.

A stronger version:

> Spare turns an unused computer into one useful machine. Install it, choose what you need, and Spare prepares, runs and maintains the service locally.

A more conceptual version:

> Spare gives old computers a new job.

A developer facing version:

> Spare is a lightweight runtime for turning ordinary computers into dependable local appliances.

The strongest homepage framing may be:

```text
Give an old computer
one useful job.

Spare profiles the machine, recommends what it can run, installs the service and keeps it working.

No server setup.
No Docker knowledge.
No public exposure required.
```

## Mana

The deeper opportunity is not “making self hosting easier.”

Spare can create a new abstraction between a personal computer and a cloud server:

```text
Computer
A general machine operated by a person

Server
Infrastructure operated by an administrator

Spare
A machine given one understandable responsibility
```

That middle category barely has a familiar consumer language today.

The breakthrough is **role assignment**.

You are not installing software onto an old laptop. You are saying:

```text
You are now the file machine.
You are now the agent machine.
You are now the archive machine.
```

That can become the fundamental interaction model for a future where homes and small companies own several imperfect computers, while AI agents continuously need local storage, execution and private tools. Spare becomes the layer that matches idle machines to useful responsibilities.