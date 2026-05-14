// Simple Redis-based job queue for the demo app.
import redis from "./redis.js";

const QUEUE_NAME = "email_jobs";

/**
 * Enqueue a job to the email_jobs queue.
 * @param {object} job - Job payload (type, to, subject, etc.)
 * @returns {Promise<number>} Queue length after push.
 */
export async function enqueue(job) {
  const payload = JSON.stringify({
    ...job,
    enqueuedAt: new Date().toISOString(),
  });
  const len = await redis.lpush(QUEUE_NAME, payload);
  console.log(`[queue] enqueued ${job.type} job, queue length: ${len}`);
  return len;
}

export { QUEUE_NAME };
