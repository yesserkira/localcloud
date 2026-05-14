// Worker process — polls Redis email_jobs queue and sends emails.
import redis from "../queue/redis.js";
import { QUEUE_NAME } from "../queue/jobs.js";
import { sendWelcomeEmail } from "./email.js";

const POLL_INTERVAL_MS = 1000;

async function processJob(raw) {
  let job;
  try {
    job = JSON.parse(raw);
  } catch {
    console.error("[worker] invalid JSON in queue:", raw);
    return;
  }

  console.log(`[worker] processing ${job.type} job for ${job.to}`);

  switch (job.type) {
    case "welcome_email":
      await sendWelcomeEmail({ to: job.to, name: job.name });
      break;
    default:
      console.warn(`[worker] unknown job type: ${job.type}`);
  }
}

async function poll() {
  while (true) {
    try {
      // BRPOP blocks for up to 5 seconds waiting for a job
      const result = await redis.brpop(QUEUE_NAME, 5);
      if (result) {
        const [, raw] = result;
        await processJob(raw);
      }
    } catch (err) {
      console.error("[worker] poll error:", err.message);
      // Back off on error
      await new Promise((r) => setTimeout(r, POLL_INTERVAL_MS));
    }
  }
}

console.log(`[worker] started, listening on queue: ${QUEUE_NAME}`);
poll().catch((err) => {
  console.error("[worker] fatal:", err);
  process.exit(1);
});
