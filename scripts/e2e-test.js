#!/usr/bin/env node
// Full E2E demo test for LocalCloud.
// Prerequisites:
//   1. localcloud init --example demo-saas
//   2. localcloud up  (or docker compose up in demo/saas-app)
//   3. Agent running on 127.0.0.1:41778
//
// This script:
//   - Verifies the agent is healthy
//   - Starts a recording
//   - Triggers the signup flow through the demo app
//   - Stops the recording
//   - Verifies events were captured
//   - Exports the scenario
//   - Replays the scenario
//   - Creates a fault rule (force 500 on POST /signup)
//   - Replays again and verifies it fails under fault
//   - Cleans up the fault rule
//
// Usage: node scripts/e2e-test.js

const API_BASE = process.env.API_BASE || "http://localhost:3000";
const AGENT_BASE = process.env.AGENT_BASE || "http://127.0.0.1:41778";
const MAILPIT_API = process.env.MAILPIT_API || "http://localhost:8025";

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

async function agentPost(path, body) {
  const res = await fetch(`${AGENT_BASE}${path}`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(body),
  });
  return { status: res.status, body: await res.json() };
}

async function agentGet(path) {
  const res = await fetch(`${AGENT_BASE}${path}`);
  return { status: res.status, body: await res.json() };
}

async function agentPatch(path, body) {
  const res = await fetch(`${AGENT_BASE}${path}`, {
    method: "PATCH",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(body),
  });
  return { status: res.status, body: await res.json() };
}

async function agentDelete(path) {
  const res = await fetch(`${AGENT_BASE}${path}`, { method: "DELETE" });
  return { status: res.status, body: await res.json() };
}

async function sleep(ms) {
  return new Promise((r) => setTimeout(r, ms));
}

async function main() {
  console.log("LocalCloud E2E Test\n");

  // ─── 1. Agent health ───
  console.log("1. Agent health check");
  try {
    const { status, body } = await agentGet("/api/health");
    if (status === 200 && body.status === "ok") {
      ok(`Agent healthy (v${body.version}, run=${body.runId.slice(0, 12)})`);
    } else {
      fail("Agent health", `status=${status}`);
      process.exit(1);
    }
  } catch (err) {
    fail("Agent health", `Cannot reach agent at ${AGENT_BASE}: ${err.message}`);
    process.exit(1);
  }

  // ─── 2. Start recording ───
  const scenarioName = `e2e-${Date.now()}`;
  console.log(`\n2. Start recording: ${scenarioName}`);
  let scenarioId;
  try {
    const { status, body } = await agentPost("/api/scenarios/start", {
      name: scenarioName,
      description: "E2E test scenario",
      tags: ["e2e", "automated"],
    });
    if (status === 201 && body.id) {
      ok(`Recording started (id=${body.id.slice(0, 16)})`);
      scenarioId = body.id;
    } else {
      fail("Start recording", `status=${status} body=${JSON.stringify(body)}`);
      process.exit(1);
    }
  } catch (err) {
    fail("Start recording", err.message);
    process.exit(1);
  }

  // ─── 3. Trigger signup flow ───
  const email = `e2e-${Date.now()}@example.test`;
  console.log(`\n3. Trigger signup: ${email}`);
  try {
    const res = await fetch(`${API_BASE}/signup`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({
        email,
        name: "E2E Test User",
        password: "e2eTestSecure123",
      }),
    });
    const body = await res.json();
    if (res.status === 201 && body.id) {
      ok(`Signup succeeded (userId=${body.id})`);
    } else {
      fail("Signup", `status=${res.status}`);
    }
  } catch (err) {
    fail("Signup", err.message);
  }

  // Wait for worker to process email
  await sleep(3000);

  // ─── 4. Verify email in Mailpit ───
  console.log("\n4. Verify welcome email");
  try {
    const res = await fetch(`${MAILPIT_API}/api/v1/messages?limit=10`);
    const body = await res.json();
    const messages = body.messages || [];
    const found = messages.find((m) =>
      m.To && m.To.some((t) => t.Address === email)
    );
    if (found) {
      ok(`Welcome email captured (subject: ${found.Subject})`);
    } else {
      fail("Welcome email", `not found for ${email}`);
    }
  } catch (err) {
    fail("Welcome email", err.message);
  }

  // ─── 5. Stop recording ───
  console.log("\n5. Stop recording");
  try {
    const { status, body } = await agentPost("/api/scenarios/stop", {});
    if (status === 200 && body.id) {
      ok(`Recording stopped (events=${body.eventCount}, replayable=${body.replayableCount})`);
    } else {
      fail("Stop recording", `status=${status}`);
    }
  } catch (err) {
    fail("Stop recording", err.message);
  }

  // ─── 6. Verify events captured ───
  console.log("\n6. Verify captured events");
  try {
    const { status, body } = await agentGet(`/api/scenarios/${scenarioId}`);
    if (status === 200 && body.events && body.events.length > 0) {
      const sources = [...new Set(body.events.map((e) => e.source))];
      ok(`${body.events.length} events from sources: ${sources.join(", ")}`);
    } else {
      fail("Events captured", `status=${status}, events=${body.events?.length ?? 0}`);
    }
  } catch (err) {
    fail("Events captured", err.message);
  }

  // ─── 7. Export scenario ───
  console.log("\n7. Export scenario");
  try {
    const res = await fetch(`${AGENT_BASE}/api/scenarios/${scenarioId}/export`, {
      method: "POST",
    });
    if (res.status === 200) {
      const data = await res.json();
      if (data.format === "localcloud.scenario.v1" && data.events) {
        ok(`Export OK (format=${data.format}, events=${data.events.length}, safe=${data.redactionReport.safe})`);
      } else {
        fail("Export", "unexpected format");
      }
    } else {
      fail("Export", `status=${res.status}`);
    }
  } catch (err) {
    fail("Export", err.message);
  }

  // ─── 8. Replay scenario (should pass) ───
  console.log("\n8. Replay scenario (expect pass)");
  let replayRunId;
  try {
    const { status, body } = await agentPost(`/api/scenarios/${scenarioId}/replay`, {
      baseUrl: API_BASE,
      skipUnsafe: true, // skip POST for clean replay
    });
    if (status === 200 && body.runId) {
      ok(`Replay completed (passed=${body.passed}, failed=${body.failed}, skipped=${body.skipped})`);
      replayRunId = body.runId;
      if (body.failed > 0) {
        fail("Replay pass", `${body.failed} requests failed`);
      }
    } else {
      fail("Replay", `status=${status}`);
    }
  } catch (err) {
    fail("Replay", err.message);
  }

  // ─── 9. Verify replay run in API ───
  if (replayRunId) {
    console.log("\n9. Verify replay run");
    try {
      const { status, body } = await agentGet(`/api/replay-runs/${replayRunId}`);
      if (status === 200 && body.run) {
        ok(`Replay run stored (status=${body.run.status})`);
      } else {
        fail("Replay run", `status=${status}`);
      }
    } catch (err) {
      fail("Replay run", err.message);
    }
  }

  // ─── 10. Create fault rule ───
  console.log("\n10. Create fault rule (force 500 on POST /signup)");
  let faultRuleId;
  try {
    const { status, body } = await agentPost("/api/fault-rules", {
      name: "e2e-signup-500",
      kind: "force_http_status",
      scope: "both",
      enabled: true,
      match: { method: "POST", pathPrefix: "/signup" },
      action: { statusCode: 500, reason: "E2E fault test" },
      safety: { maxHits: 5, expiresAfter: "5m" },
    });
    if (status === 201 && body.id) {
      ok(`Fault rule created (id=${body.id.slice(0, 16)})`);
      faultRuleId = body.id;
    } else {
      fail("Create fault rule", `status=${status}`);
    }
  } catch (err) {
    fail("Create fault rule", err.message);
  }

  // ─── 11. Verify fault rule in list ───
  console.log("\n11. Verify fault rule in list");
  try {
    const { status, body } = await agentGet("/api/fault-rules");
    if (status === 200) {
      const found = (body.items || []).find((r) => r.id === faultRuleId);
      if (found && found.enabled) {
        ok(`Fault rule listed (name=${found.name}, enabled=${found.enabled})`);
      } else {
        fail("Fault rule list", "rule not found or not enabled");
      }
    } else {
      fail("Fault rule list", `status=${status}`);
    }
  } catch (err) {
    fail("Fault rule list", err.message);
  }

  // ─── 12. Replay under fault (expect failure) ───
  console.log("\n12. Replay under fault (expect signup to fail)");
  try {
    const { status, body } = await agentPost(`/api/scenarios/${scenarioId}/replay`, {
      baseUrl: API_BASE,
      skipUnsafe: false,
    });
    if (status === 200 && body.runId) {
      if (body.failed > 0) {
        ok(`Replay correctly shows failure under fault (passed=${body.passed}, failed=${body.failed})`);
      } else {
        fail("Replay under fault", "expected failures but all passed");
      }
    } else {
      fail("Replay under fault", `status=${status}`);
    }
  } catch (err) {
    fail("Replay under fault", err.message);
  }

  // ─── 13. Disable fault rule ───
  if (faultRuleId) {
    console.log("\n13. Disable fault rule");
    try {
      const { status } = await agentPatch(`/api/fault-rules/${faultRuleId}`, {
        enabled: false,
      });
      if (status === 200) {
        ok("Fault rule disabled");
      } else {
        fail("Disable fault rule", `status=${status}`);
      }
    } catch (err) {
      fail("Disable fault rule", err.message);
    }
  }

  // ─── 14. Delete fault rule ───
  if (faultRuleId) {
    console.log("\n14. Delete fault rule");
    try {
      const { status } = await agentDelete(`/api/fault-rules/${faultRuleId}`);
      if (status === 200) {
        ok("Fault rule deleted");
      } else {
        fail("Delete fault rule", `status=${status}`);
      }
    } catch (err) {
      fail("Delete fault rule", err.message);
    }
  }

  // ─── 15. Verify services endpoint ───
  console.log("\n15. Verify services endpoint");
  try {
    const { status, body } = await agentGet("/api/services");
    if (status === 200) {
      const count = (body.services || []).length;
      ok(`Services endpoint OK (${count} services)`);
    } else {
      fail("Services", `status=${status}`);
    }
  } catch (err) {
    fail("Services", err.message);
  }

  // ─── Summary ───
  console.log(`\n${"═".repeat(40)}`);
  console.log(`E2E Results: ${passed} passed, ${failed} failed`);
  console.log(`${"═".repeat(40)}`);
  process.exit(failed > 0 ? 1 : 0);
}

main();
