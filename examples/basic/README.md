# Basic CRUD

Demonstrates standard REST operations with [JSONPlaceholder](https://jsonplaceholder.typicode.com):

- **GET** single and list endpoints
- **POST** to create a resource
- **PUT** and **PATCH** to update
- **DELETE** to remove
- Query parameter filtering (`? userId = 1`)
- Captures (`>>>capture` block)
- Common assertion operators: `==`, `exists`, `type`, `contains`

## Run

```bash
hitspec run examples/basic/crud.http
```
