# Basic CRUD

Demonstrates standard REST operations (modelled on [JSONPlaceholder](https://jsonplaceholder.typicode.com)):

- **GET** single and list endpoints
- **POST** to create a resource
- **PUT** and **PATCH** to update
- **DELETE** to remove
- Query parameter filtering (`? userId = 1`)
- Captures (`>>>capture` block)
- Common assertion operators: `==`, `exists`, `type`, `contains`

## Run (offline)

Every request has a `>>>mock` block, so `hitspec mock` can serve the file and
`hitspec run` can test it, with no internet access:

```bash
hitspec mock examples/basic/crud.http &      # serves http://localhost:3000
hitspec run examples/basic/crud.http
kill %1
```

The mock replays canned responses (it does not run your assertions or any real
logic). To test the real service, set `@baseUrl = https://jsonplaceholder.typicode.com` in the file.

