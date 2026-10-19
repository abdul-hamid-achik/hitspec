# Mock server

Demonstrates the `>>>mock` block: `hitspec mock` serves each block verbatim, so
this file is both the spec and the fake API it is tested against.

## Run

```bash
hitspec mock examples/mock/mock.http --port 3000 &
curl http://localhost:3000/users/1
hitspec run examples/mock/mock.http
kill %1
```

The last request (`POST /users`) has no mock block: the server infers a `201`
response and the body from its `expect` lines.
