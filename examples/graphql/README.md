# GraphQL Examples

Demonstrates how to write and test GraphQL queries with hitspec.

## Features Covered

- GraphQL queries with `>>>graphql` block syntax
- Query variables with `>>>variables` block
- Asserting on nested GraphQL response paths (`body.data.country.name`)
- Parameterized queries with `$code: ID!`
- Multiple root fields in a single query

## API Used

[Countries GraphQL API](https://countries.trevorblades.com/) -- a free, public GraphQL endpoint.

## Running

```bash
hitspec run examples/graphql/graphql.http

# Run only read queries
hitspec run examples/graphql/graphql.http --tags read

# Run filter tests
hitspec run examples/graphql/graphql.http --tags filter
```

## Key Concepts

- Use `>>>graphql` / `<<<` to define the query body
- Use `>>>variables` inside the graphql block to pass variables as JSON
- Assert on `body.data.<field>` paths since GraphQL wraps responses in `data`
