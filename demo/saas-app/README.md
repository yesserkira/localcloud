# LocalCloud Demo SaaS App

A minimal SaaS signup flow used as LocalCloud's flagship demo.

## What It Does

```
POST /signup { email, name, password }
  → Insert user into Postgres
  → Enqueue welcome_email job in Redis
  → Worker picks up job
  → Worker sends email via Mailpit SMTP
  → Email appears in Mailpit UI at http://localhost:8025
```

## Stack

| Service  | Port  | Purpose              |
|----------|-------|----------------------|
| API      | 3000  | Fastify HTTP server  |
| Worker   | —     | Redis queue consumer |
| Postgres | 5432  | User storage         |
| Redis    | 6379  | Job queue            |
| Mailpit  | 8025  | Email capture (UI)   |
| Mailpit  | 1025  | Email capture (SMTP) |

## Quick Start

```bash
# Start everything
docker compose up -d

# Wait for services to be healthy
docker compose ps

# Test the signup flow
curl -s http://localhost:3000/health | jq .

curl -s -X POST http://localhost:3000/signup \
  -H "Content-Type: application/json" \
  -d '{"email":"alex@example.test","name":"Alex","password":"test1234secure"}' | jq .

# Check Mailpit for the welcome email
open http://localhost:8025

# Run the smoke test
node scripts/smoke-test.js
```

## Endpoints

| Method | Path    | Description              |
|--------|---------|--------------------------|
| GET    | /health | Health check (pg + redis) |
| POST   | /signup | Create user + send email |

## Stopping

```bash
docker compose down
docker compose down -v  # also remove volumes (resets database)
```
