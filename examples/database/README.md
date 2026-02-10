# Database Assertion Examples

Demonstrates how to verify database state alongside HTTP API responses.

## Features Covered

- `@db <connection-string>` -- Specify a database connection per request
- `>>>db` / `<<<` blocks for SQL queries with assertions
- Checking row counts, specific field values, and boolean states
- Multiple `>>>db` blocks on a single request

## Prerequisites

Create the test SQLite database:

```bash
sqlite3 ./examples/database/test.db <<'SQL'
CREATE TABLE users (
  id INTEGER PRIMARY KEY,
  name TEXT,
  email TEXT,
  active BOOLEAN DEFAULT 1
);
INSERT INTO users (name, email) VALUES ('Alice', 'alice@example.com');
SQL
```

## Running

Database assertions require the `--allow-db` flag for security:

```bash
hitspec run examples/database/database.http --allow-db

# Run only read tests
hitspec run examples/database/database.http --allow-db --tags read

# Run write tests
hitspec run examples/database/database.http --allow-db --tags write
```

## Supported Databases

- SQLite: `sqlite://./path/to/db.sqlite`
- PostgreSQL: `postgres://user:pass@host:5432/dbname`
- MySQL: `mysql://user:pass@host:3306/dbname`

## Key Concepts

- DB assertions run after the HTTP request completes
- Each `>>>db` block contains a SQL query followed by column assertions
- Use `expect <column> <operator> <value>` syntax inside `>>>db` blocks
- Always use `--allow-db` -- hitspec refuses to run DB queries without explicit consent
