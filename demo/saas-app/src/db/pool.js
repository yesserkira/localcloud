// Database connection pool for the demo app.
import pg from "pg";

const pool = new pg.Pool({
  connectionString: process.env.DATABASE_URL,
  max: 10,
});

pool.on("error", (err) => {
  console.error("Unexpected Postgres pool error:", err.message);
});

export default pool;
