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
GET    /api/v1/machine
GET    /api/v1/recipes
GET    /api/v1/instances
GET    /api/v1/instances/{id}
GET    /api/v1/events
POST   /api/v1/instances
POST   /api/v1/instances/{id}/start
POST   /api/v1/instances/{id}/stop
POST   /api/v1/instances/{id}/heartbeat
POST   /api/v1/browser-sessions
DELETE /api/v1/instances/{id}
```

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
resource guidance, configuration fields, declared permissions, and machine
compatibility.

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
