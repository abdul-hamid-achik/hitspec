# Petstore API

A real-world CRUD lifecycle modelled on the [Swagger Petstore](https://petstore.swagger.io) API.

Demonstrates:

- Request dependencies with `@depends`
- Variable captures across requests
- JSON body payloads
- Multiple assertion operators

## Run (offline)

Every request has a `>>>mock` block, so `hitspec mock` can serve the file and
`hitspec run` can test it, with no internet access:

```bash
hitspec mock examples/petstore/petstore.http &      # serves http://localhost:3000
hitspec run examples/petstore/petstore.http
kill %1
```

The mock replays canned responses (it does not run your assertions or any real
logic). To test the real service, set `@baseUrl = https://petstore.swagger.io/v2` in the file.

