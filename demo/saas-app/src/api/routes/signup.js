// Signup route — POST /signup
import bcrypt from "bcrypt";
import pool from "../db/pool.js";
import { enqueue } from "../queue/jobs.js";

const SALT_ROUNDS = 10;

/**
 * Register the signup route on the Fastify instance.
 * @param {import('fastify').FastifyInstance} app
 */
export default async function signupRoute(app) {
  app.post(
    "/signup",
    {
      schema: {
        body: {
          type: "object",
          required: ["email", "name", "password"],
          properties: {
            email: { type: "string", format: "email", maxLength: 255 },
            name: { type: "string", minLength: 1, maxLength: 255 },
            password: { type: "string", minLength: 8, maxLength: 128 },
          },
        },
        response: {
          201: {
            type: "object",
            properties: {
              id: { type: "integer" },
              email: { type: "string" },
              name: { type: "string" },
              createdAt: { type: "string" },
            },
          },
        },
      },
    },
    async (request, reply) => {
      const { email, name, password } = request.body;

      // Check for existing user
      const existing = await pool.query(
        "SELECT id FROM users WHERE email = $1",
        [email]
      );
      if (existing.rows.length > 0) {
        return reply.status(409).send({
          error: "conflict",
          message: "A user with this email already exists.",
        });
      }

      // Hash password
      const passwordHash = await bcrypt.hash(password, SALT_ROUNDS);

      // Insert user
      const result = await pool.query(
        `INSERT INTO users (email, name, password_hash)
         VALUES ($1, $2, $3)
         RETURNING id, email, name, created_at`,
        [email, name, passwordHash]
      );
      const user = result.rows[0];

      // Enqueue welcome email job
      await enqueue({
        type: "welcome_email",
        to: user.email,
        name: user.name,
        userId: user.id,
      });

      request.log.info({ userId: user.id, email: user.email }, "user signed up");

      return reply.status(201).send({
        id: user.id,
        email: user.email,
        name: user.name,
        createdAt: user.created_at,
      });
    }
  );
}
