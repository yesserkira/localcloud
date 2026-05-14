# LocalCloud

LocalCloud is a control plane for local development. You run your backend stack (Docker Compose, Postgres, Redis, whatever) and LocalCloud sits on top — recording what happens across services, letting you replay captured flows, and injecting faults to see what breaks.

```
┌─────────────────────────────────────────────────────────┐
│                    Studio UI (React)                    │
│        Timeline │ Scenarios │ Faults │ Services         │
└──────────────────────────┬──────────────────────────────┘
                           │ REST + SSE
┌──────────────────────────▼──────────────────────────────┐
│                     Agent (Go)                          │
│   HTTP Proxy │ Postgres │ Redis │ Mailpit adapters      │
│   Replay engine │ Fault engine │ Export                  │
├─────────────────────────────────────────────────────────┤
│                   SQLite (WAL mode)                     │
└─────────────────────────────────────────────────────────┘
```

## Why

Modern backend work touches a lot of moving parts — API, database, queue, workers, email, maybe some external APIs. Docker Compose gets them running, but it doesn't tell you what actually happened across the stack when you hit that signup endpoint. LocalCloud does.

## Quick Start

Requires Go 1.23+, Node.js 20+, and Docker.

```bash
git clone https://github.com/localcloud-dev/localcloud.git
cd localcloud
make build

# Set up the demo (Fastify API + Postgres + Redis + Mailpit)
./localcloud init --example demo-saas
./localcloud up
```

Then in another terminal:

```bash
# Start recording
./localcloud record --name my-first-scenario

# Trigger a signup
curl -X POST http://localhost:4000/signup \
  -H "Content-Type: application/json" \
  -d '{"email":"test@example.com","name":"Jane","password":"test1234"}'

# Stop and replay
./localcloud stop
./localcloud replay --scenario <id> --base-url http://localhost:4000 --confirm-unsafe
```

Open Studio at `http://127.0.0.1:41778` to see the timeline.

## Adapters

| Adapter | How it captures |
| --- | --- |
| HTTP services | Reverse proxy |
| PostgreSQL | Polls an audit trigger table |
| Redis | `MONITOR` command |
| Mailpit | REST API polling |

## Fault Injection

Force a 500, add latency, drop a request, mutate a response body, simulate a timeout. Faults have safety limits (max hits, auto-expiry) and only affect traffic going through the proxy.

```bash
./localcloud fault create --name break-signup \
  --kind force_http_status --path-prefix /signup \
  --status-code 500 --max-hits 5
```

## Building

```bash
make build          # build CLI
make test           # run Go tests
make studio-build   # build the UI
make check          # all of the above
```

## Docs

- [Architecture](docs/architecture.md)
- [CLI Reference](docs/cli.md)

## License

[MIT](LICENSE)
