# Changelog

## 1.0.0 — 2026-05-14

Initial release.

- HTTP proxy, Postgres, Redis, and Mailpit adapters
- Scenario recording and replay with diff comparison
- Fault injection (delay, force status, drop, timeout, mutate response)
- Studio UI with live timeline, scenario management, and fault controls
- CLI: `init`, `up`, `down`, `record`, `stop`, `replay`, `export`, `doctor`, `fault`
- Automatic header/body redaction for sensitive fields
- Demo app: Fastify signup → Postgres → Redis job → Mailpit email
- CI + multi-platform release builds
