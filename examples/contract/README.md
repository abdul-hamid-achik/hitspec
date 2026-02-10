# Contract Testing Examples

Demonstrates how to verify API responses against an expected contract (schema).

## Features Covered

- Type assertions (`expect body.id type number`) to enforce response shape
- `in` operator for enum validation (`expect body.status in ["placed", "approved"]`)
- Error response contract validation
- Field existence checks to ensure required fields are present
- Chained requests with `@depends` to test create-then-read flows

## API Used

[Swagger Petstore](https://petstore.swagger.io/) -- a well-known demo API with an OpenAPI spec.

## Running

```bash
hitspec run examples/contract/contract.http

# Run only store-related contract tests
hitspec run examples/contract/contract.http --tags store

# Run error contract tests
hitspec run examples/contract/contract.http --tags error

# Generate coverage against an OpenAPI spec
hitspec run examples/contract/contract.http --coverage --openapi https://petstore.swagger.io/v2/swagger.json
```

## Key Concepts

- Use `type` assertions to enforce that fields match their schema types
- Use `in [...]` for enum fields to ensure values stay within allowed sets
- Use `exists` / `!exists` to verify required vs optional fields
- Test error responses too -- they are part of the API contract
- Combine with `--coverage --openapi` to measure how much of the spec your tests cover
