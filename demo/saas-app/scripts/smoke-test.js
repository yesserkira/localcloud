#!/usr/bin/env node
// Smoke test for the demo SaaS app.
// Expects the stack to be running (docker compose up).
// Usage: node scripts/smoke-test.js

const API_BASE = process.env.API_BASE || "http://localhost:3000";
const MAILPIT_API = process.env.MAILPIT_API || "http://localhost:8025";

async function main() {
  let passed = 0;
  let failed = 0;

  function ok(name) {
    console.log(`  ✓ ${name}`);
    passed++;
  }
  function fail(name, reason) {
    console.error(`  ✗ ${name}: ${reason}`);
    failed++;
  }

  console.log("Demo SaaS smoke test\n");

  // 1. Health check
  console.log("1. Health check");
  try {
    const res = await fetch(`${API_BASE}/health`);
    const body = await res.json();
    if (res.status === 200 && body.status === "ok") {
      ok("GET /health returns 200");
    } else {
      fail("GET /health", `status=${res.status} body=${JSON.stringify(body)}`);
    }
  } catch (err) {
    fail("GET /health", err.message);
  }

  // 2. Signup
  const email = `smoke-${Date.now()}@example.test`;
  console.log("\n2. Signup");
  let userId;
  try {
    const res = await fetch(`${API_BASE}/signup`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({
        email,
        name: "Smoke Test",
        password: "test1234secure",
      }),
    });
    const body = await res.json();
    if (res.status === 201 && body.id && body.email === email) {
      ok(`POST /signup creates user (id=${body.id})`);
      userId = body.id;
    } else {
      fail("POST /signup", `status=${res.status} body=${JSON.stringify(body)}`);
    }
  } catch (err) {
    fail("POST /signup", err.message);
  }

  // 3. Duplicate signup blocked
  console.log("\n3. Duplicate signup");
  try {
    const res = await fetch(`${API_BASE}/signup`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({
        email,
        name: "Duplicate",
        password: "test1234secure",
      }),
    });
    if (res.status === 409) {
      ok("Duplicate signup returns 409");
    } else {
      fail("Duplicate signup", `expected 409, got ${res.status}`);
    }
  } catch (err) {
    fail("Duplicate signup", err.message);
  }

  // 4. Wait for worker to process email, then check Mailpit
  console.log("\n4. Welcome email in Mailpit");
  // Give the worker a few seconds to process the job
  await new Promise((r) => setTimeout(r, 3000));
  try {
    const res = await fetch(`${MAILPIT_API}/api/v1/messages?limit=10`);
    const body = await res.json();
    const messages = body.messages || [];
    const found = messages.find((m) =>
      m.To && m.To.some((t) => t.Address === email)
    );
    if (found) {
      ok(`Welcome email found in Mailpit (subject: ${found.Subject})`);
    } else {
      fail("Welcome email", `not found for ${email} in ${messages.length} messages`);
    }
  } catch (err) {
    fail("Welcome email", err.message);
  }

  // Summary
  console.log(`\n${passed} passed, ${failed} failed`);
  process.exit(failed > 0 ? 1 : 0);
}

main();
