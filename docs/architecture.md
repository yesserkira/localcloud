# Architecture

## Overview

LocalCloud is a local-only control plane that observes your development stack and provides four capabilities: **Record**, **Visualize**, **Replay**, **Fault-test**.

It does not replace Docker, Postgres, Redis, or any service you use. It sits above your stack and captures what happens.

## Components

```
┌──────────────────────────────────────────────────────────────┐
│                         Studio UI                            │
│               React + Vite + TanStack Query                  │
│         (Timeline, Scenarios, Replay, Faults, Services)      │
└────────────────────────────┬─────────────────────────────────┘
                             │ REST + SSE
┌────────────────────────────▼─────────────────────────────────┐
│                        Studio API                            │
│                   net/http (Go stdlib)                        │
│     /api/health, /events, /scenarios, /fault-rules, etc.     │
└────────────────────────────┬─────────────────────────────────┘
                             │
┌────────────────────────────▼─────────────────────────────────┐
│                          Agent                               │
│   Lifecycle │ Event Bus │ Sink │ Recording │ Config           │
├──────────────────────────────────────────────────────────────┤
│  Adapters                                                    │
│  ┌──────────┐ ┌──────────┐ ┌──────────┐ ┌──────────┐        │
│  │HTTP Proxy│ │ Postgres │ │  Redis   │ │ Mailpit  │        │
│  │(reverse) │ │(audit tbl)│ │(MONITOR) │ │(REST API)│        │
│  └──────────┘ └──────────┘ └──────────┘ └──────────┘        │
├──────────────────────────────────────────────────────────────┤
│  Engines                                                     │
│  ┌──────────┐ ┌──────────┐ ┌──────────┐                     │
│  │ Replay   │ │  Fault   │ │ Export   │                     │
│  │ Engine   │ │ Engine   │ │ Engine   │                     │
│  └──────────┘ └──────────┘ └──────────┘                     │
├──────────────────────────────────────────────────────────────┤
│                      SQLite (WAL)                            │
│  events │ scenarios │ replay_runs │ fault_rules │ services   │
└──────────────────────────────────────────────────────────────┘
```

## Data Flow

1. Your application sends requests through the HTTP proxy or interacts with Postgres/Redis/Mailpit.
2. Each adapter captures the activity and emits a `TimelineEvent` to the agent sink.
3. The sink validates, tags with scenario ID (if recording), and inserts into SQLite.
4. The event bus publishes the event for SSE broadcast to Studio clients.
5. Studio receives events via SSE and updates the timeline in real time.

## Key Design Decisions

- **Local-only**: All data stays on the developer's machine. No cloud, no accounts, no telemetry.
- **SQLite**: Single-file database with WAL mode. No external database dependency.
- **Compiled-in adapters**: No plugin system. Adapters are Go packages compiled into the binary.
- **HTTP reverse proxy**: Captures requests without modifying the target application.
- **Redaction by default**: Sensitive headers and body fields are redacted before storage.

## Directory Structure

```
cmd/localcloud/          CLI entrypoint
internal/
  agent/                 Agent lifecycle and recording state
  api/                   Studio REST API and SSE server
  adapters/              Service-specific capture adapters
    postgres/            Polls _localcloud_audit table
    redis/               MONITOR command over raw TCP
    mailpit/             Polls Mailpit REST API
  config/                localcloud.yml loader and validator
  docker/                Docker Compose controller
  eventbus/              In-process pub/sub for event fanout
  fault/                 Fault rule matcher and injection engine
  id/                    ULID generation with typed prefixes
  proxy/                 HTTP reverse proxy with capture
  redaction/             Header/body/secret redaction
  replay/                Replay plan generation and execution
  scenario/              Scenario export engine
  storage/               SQLite schema, migrations, repositories
  timeline/              Canonical event and model types
studio/                  React Studio UI (Vite + TypeScript)
demo/saas-app/           Flagship demo application
scripts/                 E2E test scripts
docs/                    Documentation
```

## Storage Schema

SQLite tables:

| Table | Purpose |
| --- | --- |
| `events` | All captured timeline events |
| `scenarios` | Recorded scenario metadata |
| `replay_runs` | Replay execution results |
| `fault_rules` | Fault injection rules |
| `services` | Service health status |
| `adapter_status` | Adapter connection state |
| `config_snapshots` | Config snapshots by hash |

All timestamps stored as Unix milliseconds. JSON fields stored as TEXT with `_json` suffix.

## Event Model

The `TimelineEvent` is the canonical fact type. Key fields:

- `id`: ULID with `evt_` prefix
- `runId`: Agent run session ID
- `scenarioId`: Set when recording is active
- `source`: Which adapter produced it (http_proxy, postgres, redis, mailpit)
- `action`: What happened (http.request, postgres.insert, redis.enqueue, email.captured)
- `correlationId`: Links events from the same user request across services
- `request`/`response`: Redacted HTTP data (for proxy events)
- `rawPayload`: Redacted non-HTTP payload (for database/queue events)
- `faults`: Applied fault annotations

## Adapter Model

Each adapter implements:

```go
type Adapter interface {
    Name() string
    Configure(config AdapterConfig) error
    Start(ctx context.Context, sink EventSink) error
    Stop(ctx context.Context) error
    Status() *timeline.AdapterStatus
}
```

Adapters push events into the `EventSink`, which validates, inserts, and publishes.
