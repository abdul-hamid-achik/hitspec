# GraphQL Examples

Demonstrates how to write and test GraphQL queries with hitspec.

## Features Covered

- GraphQL queries with `>>>graphql` block syntax
- Query variables with `>>>variables` block
- Asserting on nested GraphQL response paths (`body.data.country.name`)
- Parameterized queries with `$code: ID!`
- Multiple root fields in a single query

## API Used

Modelled on the [Countries GraphQL API](https://countries.trevorblades.com/), a
free, public GraphQL endpoint. The file carries `>>>mock` blocks, so it runs
offline against the built-in mock server. The mock routes on method + path only
and cannot tell two GraphQL queries apart, so each operation posts to its own
path (`/graphql/countries`, `/graphql/country`, ...). A real server has one
`/graphql` endpoint: to use the Countries API, set
`@baseUrl = https://countries.trevorblades.com` and change the URLs back to
`/graphql`.

## Running

```bash
hitspec mock examples/graphql/graphql.http &    # serves http://localhost:3000
hitspec run examples/graphql/graphql.http

# Run only read queries
hitspec run examples/graphql/graphql.http --tags read

# Run filter tests
hitspec run examples/graphql/graphql.http --tags filter

kill %1
```

## Key Concepts

- Use `>>>graphql` / `<<<` to define the query body
- Use `>>>variables` inside the graphql block to pass variables as JSON
- Assert on `body.data.<field>` paths since GraphQL wraps responses in `data`
