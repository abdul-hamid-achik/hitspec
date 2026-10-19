-- Seed data for database.http. Create the database once:
--   cd examples/database && sqlite3 test.db < seed.sql
DROP TABLE IF EXISTS users;
CREATE TABLE users (
  id INTEGER PRIMARY KEY,
  name TEXT,
  email TEXT,
  active BOOLEAN DEFAULT 1
);
INSERT INTO users (name, email) VALUES ('Alice', 'alice@example.com');
