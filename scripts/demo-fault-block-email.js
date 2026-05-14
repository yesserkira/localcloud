#!/usr/bin/env node
// scripts/demo-fault-block-email.js
//
// Step 44: Demonstrates blocking email delivery via fault injection.
//
// The demo worker sends email via SMTP (not HTTP), so we cannot directly
// fault the email send. Instead, we fault the signup HTTP endpoint to
// return 503, which prevents the Redis job from being enqueued, which
// prevents the welcome email from being sent.
//
// Usage:
//   node scripts/demo-fault-block-email.js [--agent http://127.0.0.1:41778] [--proxy http://localhost:4000]
//
// Prerequisites:
//   localcloud up (agent + demo stack running)

const AGENT = process.argv.includes("--agent")
  ? process.argv[process.argv.indexOf("--agent") + 1]
  : "http://127.0.0.1:41778";

const PROXY = process.argv.includes("--proxy")
  ? process.argv[process.argv.indexOf("--proxy") + 1]
  : "http://localhost:4000";

let passed = 0;
let failed = 0;

async function step(label, fn) {
  process.stdout.write(`  ${label} ... `);
  try {
    await fn();
    console.log("OK");
    passed++;
  } catch (err) {
    console.log(`FAIL: ${err.message}`);
    failed++;
  }
}

function assert(cond, msg) {
  if (!cond) throw new Error(msg);
}

async function main() {
  console.log("=== Demo: Block Email Delivery via Fault Injection ===\n");
  console.log(`Agent:  ${AGENT}`);
  console.log(`Proxy:  ${PROXY}\n`);

  let faultId;

  // ── 1. Verify agent is healthy ──
  await step("1. Agent health check", async () => {
    const res = await fetch(`${AGENT}/api/health`);
    assert(res.ok, `health returned ${res.status}`);
  });

  // ── 2. Signup succeeds normally ──
  await step("2. Normal signup succeeds", async () => {
    const email = `before-fault-${Date.now()}@example.test`;
    const res = await fetch(`${PROXY}/signup`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ email, name: "Before Fault", password: "test1234" }),
    });
    assert(res.status === 201, `expected 201, got ${res.status}`);
  });

  // ── 3. Wait for email to arrive in Mailpit ──
  await step("3. Welcome email arrives (Mailpit)", async () => {
    await new Promise((r) => setTimeout(r, 3000));
    const res = await fetch("http://localhost:8025/api/v1/messages?limit=1");
    assert(res.ok, `mailpit returned ${res.status}`);
    const data = await res.json();
    assert(data.messages && data.messages.length > 0, "no messages in Mailpit");
  });

  // ── 4. Create fault rule: force 503 on POST /signup ──
  await step("4. Create fault: force 503 on POST /signup", async () => {
    const res = await fetch(`${AGENT}/api/fault-rules`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({
        name: "block-email-demo",
        kind: "force_http_status",
        scope: "live",
        enabled: true,
        match: {
          service: "api",
          method: "POST",
          pathPrefix: "/signup",
        },
        action: {
          statusCode: 503,
          reason: "Fault injected: signup blocked to prevent email delivery",
        },
        safety: {
          maxHits: 10,
          expiresAfter: "5m",
        },
      }),
    });
    assert(res.ok || res.status === 201, `create fault returned ${res.status}`);
    const body = await res.json();
    faultId = body.id;
    assert(faultId, "no fault ID returned");
    console.log(`(id: ${faultId})`);
    process.stdout.write("       ");
  });

  // ── 5. Count Mailpit messages before faulted signup ──
  let mailCountBefore = 0;
  await step("5. Count emails before faulted signup", async () => {
    const res = await fetch("http://localhost:8025/api/v1/messages?limit=100");
    assert(res.ok, `mailpit returned ${res.status}`);
    const data = await res.json();
    mailCountBefore = data.total || (data.messages ? data.messages.length : 0);
  });

  // ── 6. Signup fails with 503 (fault active) ──
  await step("6. Faulted signup returns 503", async () => {
    const email = `faulted-${Date.now()}@example.test`;
    const res = await fetch(`${PROXY}/signup`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ email, name: "Faulted User", password: "test1234" }),
    });
    assert(res.status === 503, `expected 503, got ${res.status}`);
    const faultHeader = res.headers.get("x-localcloud-fault");
    assert(faultHeader === "block-email-demo", `expected fault header, got: ${faultHeader}`);
  });

  // ── 7. No new email in Mailpit (email was blocked) ──
  await step("7. No new email sent (blocked)", async () => {
    await new Promise((r) => setTimeout(r, 3000));
    const res = await fetch("http://localhost:8025/api/v1/messages?limit=100");
    assert(res.ok, `mailpit returned ${res.status}`);
    const data = await res.json();
    const mailCountAfter = data.total || (data.messages ? data.messages.length : 0);
    assert(
      mailCountAfter === mailCountBefore,
      `expected ${mailCountBefore} emails, got ${mailCountAfter} (email was not blocked!)`
    );
  });

  // ── 8. Verify fault hit count incremented ──
  await step("8. Fault hit count incremented", async () => {
    const res = await fetch(`${AGENT}/api/fault-rules`);
    assert(res.ok, `list faults returned ${res.status}`);
    const rules = await res.json();
    const rule = rules.find((r) => r.id === faultId);
    assert(rule, "fault rule not found");
    assert(rule.hitCount > 0, `expected hitCount > 0, got ${rule.hitCount}`);
  });

  // ── 9. Disable the fault ──
  await step("9. Disable fault rule", async () => {
    const res = await fetch(`${AGENT}/api/fault-rules/${faultId}`, {
      method: "PATCH",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ enabled: false }),
    });
    assert(res.ok, `disable fault returned ${res.status}`);
  });

  // ── 10. Signup works again ──
  await step("10. Signup works again after disabling fault", async () => {
    const email = `after-fault-${Date.now()}@example.test`;
    const res = await fetch(`${PROXY}/signup`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ email, name: "After Fault", password: "test1234" }),
    });
    assert(res.status === 201, `expected 201, got ${res.status}`);
  });

  // ── 11. Email delivery resumes ──
  await step("11. Welcome email sent again", async () => {
    await new Promise((r) => setTimeout(r, 3000));
    const res = await fetch("http://localhost:8025/api/v1/messages?limit=100");
    assert(res.ok, `mailpit returned ${res.status}`);
    const data = await res.json();
    const mailCountFinal = data.total || (data.messages ? data.messages.length : 0);
    assert(
      mailCountFinal > mailCountBefore,
      `expected more emails than ${mailCountBefore}, got ${mailCountFinal}`
    );
  });

  // ── 12. Clean up: delete the fault rule ──
  await step("12. Delete fault rule", async () => {
    const res = await fetch(`${AGENT}/api/fault-rules/${faultId}`, {
      method: "DELETE",
    });
    assert(res.ok, `delete fault returned ${res.status}`);
  });

  // ── Summary ──
  console.log(`\n${"=".repeat(50)}`);
  console.log(`Passed: ${passed}  Failed: ${failed}`);
  if (failed > 0) {
    console.log("\nThe demo shows that fault injection can block email delivery");
    console.log("by preventing the signup request from reaching the API.");
    process.exit(1);
  } else {
    console.log("\nEmail delivery was successfully blocked by fault injection.");
    console.log("The signup → Postgres → Redis → Worker → Mailpit chain was");
    console.log("interrupted at the HTTP proxy layer, proving the fault engine");
    console.log("prevents downstream side effects.");
  }
}

main().catch((err) => {
  console.error("Fatal:", err);
  process.exit(1);
});
