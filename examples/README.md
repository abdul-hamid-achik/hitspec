# Examples

Every example runs **offline**: each request carries a `>>>mock` block, so
`hitspec mock` serves the file on `http://localhost:3000` and `hitspec run`
tests it, with no internet access. CI runs all of them on every push.

```bash
hitspec mock examples/basic/crud.http &     # terminal 1 (or background it)
hitspec run examples/basic/crud.http        # terminal 2
kill %1
```

| Example | Shows |
|---------|-------|
| [basic](basic) | CRUD requests, captures, common assertions |
| [auth-flow](auth-flow) | bearer, basic, API key, digest, chained login |
| [conditional](conditional) | `@if` / `@unless` |
| [contract](contract) | type and enum assertions, error contracts |
| [database](database) | `>>>db` assertions (needs `sqlite3` and `--allow-db`) |
| [graphql](graphql) | `>>>graphql` / `>>>variables` |
| [mock](mock) | the `>>>mock` block itself |
| [petstore](petstore) | a CRUD lifecycle with `@depends` |
| [retry](retry) | `@retry`, `@retryOn`, `@waitFor` |
| [stress-test](stress-test) | `--stress` mode and `@stress.*` directives |

The mock server answers with the first block matching a method + path and does
not validate what it receives (credentials, bodies), so these runs show how
hitspec behaves, not whether a real API is correct. Each file's header says
which real service to point `@baseUrl` at.
