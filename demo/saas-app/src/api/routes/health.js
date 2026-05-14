// Health check route — GET /health
import pool from "../db/pool.js";
import redis from "../queue/redis.js";

/**
 * Register the health route on the Fastify instance.
 * @param {import('fastify').FastifyInstance} app
 */
export default async function healthRoute(app) {
  app.get("/health", async (request, reply) => {
    const checks = {};

    // Postgres check
    try {
      await pool.query("SELECT 1");
      checks.postgres = "ok";
    } catch {
      checks.postgres = "error";
    }

    // Redis check
    try {
      await redis.ping();
      checks.redis = "ok";
    } catch {
      checks.redis = "error";
    }

    const allOk = Object.values(checks).every((v) => v === "ok");

    return reply.status(allOk ? 200 : 503).send({
      status: allOk ? "ok" : "degraded",
      checks,
      timestamp: new Date().toISOString(),
    });
  });
}
