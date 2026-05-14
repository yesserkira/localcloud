# CLI Reference

## Global Flags

All commands accept `--help` or `-h` for usage information.

## Commands

### `localcloud init`

Create a `localcloud.yml` configuration file and project scaffolding.

```
localcloud init [--example demo-saas] [--config path] [--force]
```

| Flag | Default | Description |
| --- | --- | --- |
| `--example` | _(none)_ | Initialize with a bundled example (`demo-saas`) |
| `--config` | `localcloud.yml` | Output config file path |
| `--force` | `false` | Overwrite existing config |

### `localcloud up`

Start the LocalCloud agent, adapters, and optionally the Docker Compose stack.

```
localcloud up [--config path] [--no-compose]
```

| Flag | Default | Description |
| --- | --- | --- |
| `--config` | `localcloud.yml` | Config file path |
| `--no-compose` | `false` | Skip Docker Compose up |

The agent runs in the foreground. Press `Ctrl+C` to stop.

### `localcloud down`

Stop the Docker Compose stack.

```
localcloud down [--config path]
```

### `localcloud status`

Show the status of Docker Compose services.

```
localcloud status [--config path]
```

### `localcloud record`

Start recording a named scenario. All events captured while recording is active are tagged with the scenario ID.

```
localcloud record --name <name> [--desc <description>] [--tags <comma-separated>] [--addr host:port]
```

| Flag | Default | Description |
| --- | --- | --- |
| `--name` | _(required)_ | Scenario name (must be unique) |
| `--desc` | `""` | Description |
| `--tags` | `""` | Comma-separated tags |
| `--addr` | `127.0.0.1:41778` | Agent API address |

### `localcloud stop`

Stop the active recording.

```
localcloud stop [--addr host:port]
```

### `localcloud replay`

Replay a scenario's captured HTTP requests against a target.

```
localcloud replay --scenario <id> --base-url <url> [--skip-unsafe] [--confirm-unsafe] [--addr host:port]
```

| Flag | Default | Description |
| --- | --- | --- |
| `--scenario` | _(required)_ | Scenario ID to replay |
| `--base-url` | _(required)_ | Target base URL |
| `--skip-unsafe` | `false` | Skip POST/PUT/DELETE/PATCH requests |
| `--confirm-unsafe` | `false` | Allow unsafe methods |
| `--addr` | `127.0.0.1:41778` | Agent API address |

Exit code 1 if any replay request fails.

### `localcloud export`

Export a scenario as portable JSON.

```
localcloud export --scenario <id> [--output <file>] [--addr host:port]
```

| Flag | Default | Description |
| --- | --- | --- |
| `--scenario` | _(required)_ | Scenario ID to export |
| `--output` | _(stdout)_ | Output file path |
| `--addr` | `127.0.0.1:41778` | Agent API address |

### `localcloud doctor`

Diagnose configuration, Docker availability, port conflicts, and network binding.

```
localcloud doctor [--config path]
```

Checks performed:
- Config file exists and parses
- Config validates
- Docker daemon available
- `docker compose` available
- Studio port not in use
- Proxy ports not in use
- All ports bind to loopback only

### `localcloud fault`

Manage fault injection rules. Subcommands:

#### `localcloud fault list`

```
localcloud fault list [--addr host:port]
```

#### `localcloud fault create`

```
localcloud fault create --name <name> --kind <kind> [flags]
```

| Flag | Default | Description |
| --- | --- | --- |
| `--name` | _(required)_ | Rule name |
| `--kind` | _(required)_ | `delay_response`, `force_http_status`, `drop_outbound_request`, `mutate_json_response`, `simulate_timeout` |
| `--scope` | `both` | `live`, `replay`, or `both` |
| `--service` | `""` | Target service |
| `--method` | `""` | HTTP method filter |
| `--path-prefix` | `""` | Path prefix filter |
| `--status-code` | `0` | Status code (for `force_http_status`) |
| `--delay-ms` | `0` | Delay in ms (for `delay_response`/`simulate_timeout`) |
| `--reason` | `""` | Error reason |
| `--max-hits` | `0` | Safety: max hits (0 = unlimited) |
| `--expires-after` | `""` | Safety: auto-expire duration (e.g. `30m`, `1h`) |

#### `localcloud fault enable <id>`

Enable a fault rule by ID.

#### `localcloud fault disable <id>`

Disable a fault rule by ID.

#### `localcloud fault delete <id>`

Delete a fault rule by ID.

### `localcloud studio`

Open the Studio dashboard in a browser. _(Not yet implemented.)_

### `localcloud version`

Print the version string.
