# Database Assertion Examples

Demonstrates how to verify database state alongside HTTP API responses.

## Features Covered

- `@db <connection-string>` -- Specify a database connection per request
- `>>>db` / `<<<` blocks for SQL queries with assertions
- Checking row counts, specific field values, and boolean states
- Multiple `>>>db` blocks on a single request

## Prerequisites

The `sqlite3` CLI, to create the test database from `seed.sql` (the HTTP side
is mocked, so no internet is needed):

```bash
cd examples/database
sqlite3 test.db < seed.sql
```

`database.http` refers to the database as `sqlite://./test.db`, relative to the
directory you run hitspec from, so run the commands below from
`examples/database/`.

## Running

Database assertions require the `--allow-db` flag for security:

```bash
hitspec mock database.http &                 # serves http://localhost:3000
hitspec run database.http --allow-db

# Run only read tests
hitspec run database.http --allow-db --tags read

# Run write tests
hitspec run database.http --allow-db --tags write

kill %1
```

Set `@baseUrl = https://jsonplaceholder.typicode.com` to use the real API
instead of the mock.

## Supported Databases

- SQLite: `sqlite://./path/to/db.sqlite`
- PostgreSQL: `postgres://user:pass@host:5432/dbname`
- MySQL: `mysql://user:pass@host:3306/dbname`

## Key Concepts

- DB assertions run after the HTTP request completes
- Each `>>>db` block contains a SQL query followed by column assertions
- Use `expect <column> <operator> <value>` syntax inside `>>>db` blocks
- Always use `--allow-db` -- hitspec refuses to run DB queries without explicit consent
