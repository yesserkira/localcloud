// Demo SaaS API server — Fastify
import Fastify from "fastify";
import signupRoute from "./routes/signup.js";
import healthRoute from "./routes/health.js";

const app = Fastify({
  logger: {
    level: "info",
    transport:
      process.env.NODE_ENV !== "production"
        ? { target: "pino-pretty", options: { translateTime: "HH:MM:ss Z" } }
        : undefined,
  },
});

// Register routes
await app.register(signupRoute);
await app.register(healthRoute);

// Global error handler
app.setErrorHandler((error, request, reply) => {
  request.log.error(error);

  if (error.validation) {
    return reply.status(400).send({
      error: "validation_error",
      message: error.message,
    });
  }

  return reply.status(500).send({
    error: "internal_error",
    message: "An unexpected error occurred.",
  });
});

// Start
const port = parseInt(process.env.PORT || "3000", 10);
const host = "0.0.0.0";

try {
  await app.listen({ port, host });
  app.log.info(`Demo API listening on ${host}:${port}`);
} catch (err) {
  app.log.fatal(err);
  process.exit(1);
}
