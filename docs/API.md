# Local API

`spared` exposes a versioned JSON API on the first available loopback port from
`7331` through `7339`. The endpoint is recorded in Spare's user state.

CLI requests use:

```http
Authorization: Bearer <local-api-token>
```

Browser requests use the HttpOnly session cookie created by
`POST /api/v1/browser-sessions` and the one-time `/auth/exchange` URL.

## Endpoints

```text
GET    /api/v1/health
GET    /api/v1/schema
GET    /api/v1/machine
GET    /api/v1/recipes
GET    /api/v1/instances
GET    /api/v1/instances/{id}
GET    /api/v1/events
GET    /api/v1/activity/stream
POST   /api/v1/instances
POST   /api/v1/instances/switch
POST   /api/v1/instances/{id}/start
POST   /api/v1/instances/{id}/stop
POST   /api/v1/instances/{id}/heartbeat
POST   /api/v1/instances/{id}/promote
POST   /api/v1/instances/{id}/configure
POST   /api/v1/browser-sessions
POST   /api/v1/desktop/backups/export
POST   /api/v1/desktop/backups/restore
POST   /api/v1/desktop/drop-files
GET    /api/v1/job-packages
POST   /api/v1/job-packages/review
POST   /api/v1/job-packages/install
DELETE /api/v1/job-packages/{id}
GET    /api/v1/job-profiles/{id}
DELETE /api/v1/instances/{id}
```

`GET /api/v1/schema` returns the checked-in JSON Schema 2020-12 document at
[`schema/api-v1.schema.json`](schema/api-v1.schema.json). The document contains
the public response models and a stable endpoint catalog. Run `make schema`
after changing a public model; tests fail when the generated reference is
stale.

`GET /api/v1/activity/stream` is an authenticated server-sent event stream of
newly committed `Event` values. Clients reconnect and refresh `GET
/api/v1/events` if a slow connection misses an event.

`POST /api/v1/instances/{id}/promote` converts a live temporary instance into
an installed instance without restarting it. The desktop quit confirmation
uses this when the user selects **Keep Drop running**.

Spare Desktop reads the protected token in its Go layer and exposes only
bounded Wails methods to React. It does not use browser pairing codes or store
the bearer token in JavaScript.

The `/api/v1/desktop/*` filesystem operations require bearer authentication.
A browser-session cookie is deliberately rejected even when it belongs to the
local dashboard. These endpoints export or restore a selected backup and copy
explicitly selected local files into an active Drop.

Creating, configuring, promoting, or removing an instance also requires the
local bearer token. A browser session may inspect status and use the bounded
start/stop controls, but it cannot select local folders or change installation
metadata.

Reviewing, installing, or uninstalling an optional job package and reading a
saved job profile require the local bearer token. A browser session may list
package status, but it cannot approve a downloaded file or alter the local job
library.

`POST /api/v1/instances/{id}/configure` validates the same manifest fields as
creation. The daemon stops and restarts a running worker, keeps the old
selected folder untouched, and restores the previous configuration if the new
worker cannot start.

`POST /api/v1/instances/switch` is the atomic one-active-job operation. It
saves the current job's profile, stops it, and starts the requested installed
job. If the new job cannot start, the daemon restores the previous job.

The job-package review response contains the publisher, package and minimum
Spare versions, SHA-256 checksum, signature status, and declared permission
statements. Installation never activates the package. A package can be
removed only while its job is inactive, and removing it does not delete
private job state or user-selected folders.

## Create Site

```json
{
  "recipeId": "site",
  "mode": "installed",
  "portMode": "auto",
  "port": 0,
  "config": {
    "path": "/absolute/path/to/public"
  }
}
```

## Create Drop

```json
{
  "recipeId": "drop",
  "mode": "installed",
  "portMode": "auto",
  "port": 0,
  "config": {
    "destination": "/absolute/path/to/received-files",
    "max-file-size": "2GB"
  }
}
```

The API validates and normalizes configuration. A size value is persisted as
bytes.

## Errors

Errors use stable codes, a plain-language message, and a concrete hint:

```json
{
  "error": {
    "code": "port_in_use",
    "message": "Port 7340 is already in use.",
    "hint": "Use `--port auto` or choose another port."
  }
}
```

The CLI preserves those codes and labels the recovery instruction:

```text
Error [port_in_use]: Port 7340 is already in use.
Recovery: Use `--port auto` or choose another port.
```

Common codes include:

- `authentication_required`
- `invalid_origin`
- `invalid_request`
- `unknown_recipe`
- `recipe_not_supported`
- `invalid_recipe_configuration`
- `role_already_exists`
- `runtime_unavailable`
- `port_in_use`
- `instance_not_found`
- `temporary_instance_not_found`
- `restart_limit_reached`

## Public models

`Machine` includes stable identity, hostname, OS, architecture, CPU, memory,
available system storage, LAN addresses, profile timestamps, and capability
flags.

`Recipe` includes ID, title, version, description, runtime, supported systems,
resource guidance, configuration fields, declared permissions, machine
compatibility, installation state, publisher, package version, checksum, and
signature status.

`Instance` includes ID, recipe ID/version, runtime, mode, desired state,
status, resolved configuration, selected data path, port preference, URLs,
health metrics, timestamps, and an optional current problem.

`Event` includes sequence ID, optional instance ID, level, kind, message,
details, and timestamp.

Instance statuses remain:

```text
starting
healthy
degraded
stopped
failed
removing
```
