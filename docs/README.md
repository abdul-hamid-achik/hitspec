# hitspec Syntax Reference

Complete reference for writing `.http` test files with hitspec.

## Table of Contents

- [Request Syntax](#request-syntax)
- [Variables](#variables)
- [Built-in Functions](#built-in-functions)
- [Headers](#headers)
- [Query Parameters](#query-parameters)
- [Request Bodies](#request-bodies)
- [Assertions](#assertions)
- [Assertion Operators](#assertion-operators)
- [Captures](#captures)
- [Metadata Directives](#metadata-directives)
- [Authentication](#authentication)
- [Dependencies](#dependencies)

---

## Request Syntax

### Basic Structure

```http
### Request Name
# @name identifier
# @description Optional description
# @tags tag1, tag2

METHOD {{baseUrl}}/path
Header-Name: Header-Value

{request body}

>>>
expect status 200
expect body.field exists
<<<

>>>capture
variableName from body.field
<<<
```

### HTTP Methods

Supported methods: `GET`, `POST`, `PUT`, `PATCH`, `DELETE`, `HEAD`, `OPTIONS`, `TRACE`, `CONNECT`

```http
GET {{baseUrl}}/users
POST {{baseUrl}}/users
PUT {{baseUrl}}/users/1
PATCH {{baseUrl}}/users/1
DELETE {{baseUrl}}/users/1
```

### Request Separator

Use `###` to separate requests. Text after `###` becomes the request name (unless overridden by `@name`).

```http
### Get all users
GET {{baseUrl}}/users

### Create user
POST {{baseUrl}}/users
```

---

## Variables

### Defining Variables

```http
@baseUrl = https://api.example.com
@token = your-api-token
@userId = 123
```

### Using Variables

Use `{{variableName}}` syntax:

```http
GET {{baseUrl}}/users/{{userId}}
Authorization: Bearer {{token}}
```

### Environment Variables

Variables can be defined per environment in `.hitspec.env.json`. See [environments.md](environments.md).

---

## Built-in Functions

Use `{{$functionName()}}` syntax:

| Function | Description | Example |
|----------|-------------|---------|
| `$uuid()` | Generate UUID v4 | `{{$uuid()}}` → `550e8400-e29b-41d4-a716-446655440000` |
| `$timestamp()` | Unix timestamp (seconds) | `{{$timestamp()}}` → `1705612800` |
| `$timestampMs()` | Unix timestamp (milliseconds) | `{{$timestampMs()}}` → `1705612800000` |
| `$now()` | Current datetime (RFC3339) | `{{$now()}}` → `2024-01-18T12:00:00Z` |
| `$isodate()` | ISO date (YYYY-MM-DD) | `{{$isodate()}}` → `2024-01-18` |
| `$date(format)` | Custom date format | `{{$date(2006-01-02)}}` → `2024-01-18` |
| `$random(min, max)` | Random integer | `{{$random(1, 100)}}` → `42` |
| `$randomString(len)` | Random alphanumeric | `{{$randomString(8)}}` → `aB3kL9mN` |
| `$randomEmail()` | Random email | `{{$randomEmail()}}` → `user_abc123@example.com` |
| `$randomAlphanumeric(len)` | Random alphanumeric | `{{$randomAlphanumeric(10)}}` → `K8mNp2qRsT` |
| `$base64(value)` | Base64 encode | `{{$base64(hello)}}` → `aGVsbG8=` |
| `$base64Decode(value)` | Base64 decode | `{{$base64Decode(aGVsbG8=)}}` → `hello` |
| `$md5(value)` | MD5 hash | `{{$md5(hello)}}` → `5d41402abc4b2a76...` |
| `$sha256(value)` | SHA256 hash | `{{$sha256(hello)}}` → `2cf24dba5fb0a30e...` |
| `$urlEncode(value)` | URL encode | `{{$urlEncode(hello world)}}` → `hello%20world` |
| `$urlDecode(value)` | URL decode | `{{$urlDecode(hello%20world)}}` → `hello world` |
| `$json(value)` | JSON passthrough | `{{$json({"key": "value"})}}` |

---

## Headers

```http
GET {{baseUrl}}/users
Content-Type: application/json
Authorization: Bearer {{token}}
Accept: application/json
X-Custom-Header: custom-value
```

---

## Query Parameters

### Inline in URL

```http
GET {{baseUrl}}/search?query=test&limit=10
```

### Explicit Syntax

```http
GET {{baseUrl}}/search
? query = test
? limit = 10
? offset = 0
```

---

## Request Bodies

### JSON Body

```http
POST {{baseUrl}}/users
Content-Type: application/json

{
  "name": "John Doe",
  "email": "john@example.com",
  "age": 30
}
```

### Form URL-Encoded

```http
POST {{baseUrl}}/login
Content-Type: application/x-www-form-urlencoded

& username = john
& password = secret123
```

### Multipart Form Data

```http
POST {{baseUrl}}/upload

>>>multipart
name = John Doe
email = john@example.com
avatar < @./photo.jpg
document < @./file.pdf
<<<
```

### GraphQL

```http
POST {{baseUrl}}/graphql
Content-Type: application/json

>>>graphql
query GetUser($id: ID!) {
  user(id: $id) {
    id
    name
    email
  }
}

>>>variables
{
  "id": "123"
}
<<<
```

### XML Body

```http
POST {{baseUrl}}/soap
Content-Type: application/xml

<?xml version="1.0" encoding="UTF-8"?>
<request>
  <action>getUser</action>
  <id>123</id>
</request>
```

---

## Assertions

Assertions validate the response. Place them inside `>>>` and `<<<` blocks:

```http
GET {{baseUrl}}/users/1

>>>
expect status 200
expect header Content-Type contains application/json
expect body.id == 1
expect body.name exists
expect body.email matches /^[\w.-]+@[\w.-]+\.\w+$/
<<<
```

### Assertion Subjects

| Subject | Description | Example |
|---------|-------------|---------|
| `status` | HTTP status code | `expect status 200` |
| `duration` | Response time (ms) | `expect duration < 1000` |
| `header <name>` | Response header | `expect header Content-Type contains json` |
| `body` | Full response body | `expect body contains "success"` |
| `body.<path>` | JSON path | `expect body.user.name == "John"` |
| `body[n]` | Array index | `expect body[0].id exists` |
| `body[n].<path>` | Nested array access | `expect body[0].user.name == "John"` |

### JSON Path Examples

```http
>>>
# Simple property
expect body.name == "John"

# Nested property
expect body.user.profile.email exists

# Array index
expect body.items[0].id == 1

# Nested array
expect body.users[0].roles[0] == "admin"
<<<
```

---

## Assertion Operators

### Equality Operators

| Operator | Syntax | Description |
|----------|--------|-------------|
| `==` | `expect status == 200` | Equals |
| `!=` | `expect body.error != null` | Not equals |

### Comparison Operators

| Operator | Syntax | Description |
|----------|--------|-------------|
| `>` | `expect body.count > 0` | Greater than |
| `>=` | `expect body.count >= 1` | Greater than or equal |
| `<` | `expect duration < 1000` | Less than |
| `<=` | `expect duration <= 500` | Less than or equal |

### String Operators

| Operator | Syntax | Description |
|----------|--------|-------------|
| `contains` | `expect body contains "success"` | Contains substring |
| `!contains` | `expect body !contains "error"` | Does not contain |
| `startsWith` | `expect body.url startsWith "https"` | Starts with prefix |
| `endsWith` | `expect body.email endsWith ".com"` | Ends with suffix |
| `matches` | `expect body.id matches /^\d+$/` | Matches regex |

### Existence Operators

| Operator | Syntax | Description |
|----------|--------|-------------|
| `exists` | `expect body.id exists` | Value is not null |
| `!exists` | `expect body.error !exists` | Value is null |

### Length Operator

| Operator | Syntax | Description |
|----------|--------|-------------|
| `length` | `expect body.items length 10` | Array/string length equals |

```http
expect body.items length 10
expect body.name length 5
```

**Note:** The length operator checks for exact equality. To verify non-empty arrays, use `exists` on the first element:
```http
expect body[0] exists
```

### Array Operators

| Operator | Syntax | Description |
|----------|--------|-------------|
| `includes` | `expect body.tags includes "admin"` | Array contains value |
| `!includes` | `expect body.tags !includes "test"` | Array does not contain |
| `in` | `expect status in [200, 201, 204]` | Value is in array |
| `!in` | `expect status !in [400, 404, 500]` | Value is not in array |

### Type Operator

| Operator | Syntax | Description |
|----------|--------|-------------|
| `type` | `expect body.items type array` | Check value type |

Valid types: `null`, `boolean`, `number`, `string`, `array`, `object`

```http
>>>
expect body.id type number
expect body.name type string
expect body.active type boolean
expect body.items type array
expect body.user type object
expect body.deleted type null
<<<
```

### Schema Validation

| Operator | Syntax | Description |
|----------|--------|-------------|
| `schema` | `expect body schema ./schema.json` | Validate against JSON Schema |

```http
>>>
expect body schema ./schemas/user.json
<<<
```

### Each Operator (Array Iteration)

| Operator | Syntax | Description |
|----------|--------|-------------|
| `each` | `expect body.items each type object` | Apply assertion to each element |

```http
>>>
# Every item must be an object
expect body.items each type object

# Every item must have an id
expect body.items each exists
<<<
```

---

## Captures

Capture values from responses for use in subsequent requests:

```http
### Login
# @name login
POST {{baseUrl}}/auth/login
Content-Type: application/json

{
  "username": "john",
  "password": "secret"
}

>>>
expect status 200
<<<

>>>capture
token from body.access_token
userId from body.user.id
<<<

### Get Profile
# @depends login
GET {{baseUrl}}/users/{{login.userId}}
Authorization: Bearer {{login.token}}
```

### Capture Sources

| Source | Syntax | Description |
|--------|--------|-------------|
| Body JSON path | `token from body.access_token` | Capture from response body |
| Header | `contentType from header Content-Type` | Capture from response header |
| Status | `code from status` | Capture status code |
| Duration | `time from duration` | Capture response time (ms) |

### Using Captured Values

Reference captured values using `{{requestName.captureName}}`:

```http
GET {{baseUrl}}/users/{{login.userId}}
Authorization: Bearer {{login.token}}
```

---

## Metadata Directives

Add metadata using `# @directive value` syntax:

### @name

Unique identifier for the request (used for captures and dependencies):

```http
# @name createUser
```

### @description

Human-readable description:

```http
# @description Creates a new user account
```

### @tags

Categorize requests for filtering:

```http
# @tags smoke, auth, critical
```

Run specific tags: `hitspec run tests/ --tags smoke`

### @skip

Skip a request with optional reason:

```http
# @skip Temporarily disabled
# @skip
```

### @only

Run only this request (useful for debugging):

```http
# @only
```

### @timeout

Request timeout in milliseconds:

```http
# @timeout 5000
```

### @retry

Number of retry attempts on failure:

```http
# @retry 3
```

### @retryDelay

Delay between retries in milliseconds:

```http
# @retryDelay 1000
```

### @retryOn

Status codes that trigger retry:

```http
# @retryOn 500, 502, 503
```

### @depends

Declare dependencies on other requests:

```http
# @depends login, setupData
```

### @if / @unless

Conditional execution:

```http
# @if {{runTests}}
# @unless {{skipAuth}}
```

---

## Authentication

### Bearer Token

```http
# @auth bearer {{token}}
GET {{baseUrl}}/protected
```

Or manually:
```http
GET {{baseUrl}}/protected
Authorization: Bearer {{token}}
```

### Basic Auth

```http
# @auth basic {{username}}, {{password}}
GET {{baseUrl}}/protected
```

### API Key (Header)

```http
# @auth apiKey X-API-Key, {{apiKey}}
GET {{baseUrl}}/protected
```

### API Key (Query String)

```http
# @auth apiKeyQuery api_key, {{apiKey}}
GET {{baseUrl}}/protected
```

### Digest Auth

```http
# @auth digest {{username}}, {{password}}
GET {{baseUrl}}/protected
```

### AWS Signature v4

```http
# @auth aws {{accessKey}}, {{secretKey}}, {{region}}, {{service}}
GET {{baseUrl}}/aws-resource
```

---

## Dependencies

Control execution order using `@depends`:

```http
### Create User
# @name createUser
POST {{baseUrl}}/users
Content-Type: application/json

{"name": "John"}

>>>capture
userId from body.id
<<<

### Update User
# @name updateUser
# @depends createUser
PUT {{baseUrl}}/users/{{createUser.userId}}
Content-Type: application/json

{"name": "John Updated"}

### Delete User
# @name deleteUser
# @depends updateUser
DELETE {{baseUrl}}/users/{{createUser.userId}}
```

### Multiple Dependencies

```http
# @depends createUser, createProfile, setupPermissions
```

### Dependency Behavior

- Requests run in topological order based on dependencies
- If a dependency fails, dependent requests are skipped
- Circular dependencies are detected and reported as errors

---

## Complete Example

```http
@baseUrl = https://api.example.com
@adminToken = secret-admin-token

### Health Check
# @name healthCheck
# @tags smoke
GET {{baseUrl}}/health

>>>
expect status 200
expect body.status == "ok"
<<<

### Login
# @name login
# @tags auth
POST {{baseUrl}}/auth/login
Content-Type: application/json

{
  "email": "test@example.com",
  "password": "password123"
}

>>>
expect status 200
expect body.token exists
expect body.user.id type number
<<<

>>>capture
token from body.token
userId from body.user.id
<<<

### Get User Profile
# @name getProfile
# @tags users
# @depends login
GET {{baseUrl}}/users/{{login.userId}}
Authorization: Bearer {{login.token}}

>>>
expect status 200
expect body.id == {{login.userId}}
expect body.email == "test@example.com"
expect duration < 500
<<<

### Update Profile
# @name updateProfile
# @tags users
# @depends getProfile
# @timeout 10000
PUT {{baseUrl}}/users/{{login.userId}}
Authorization: Bearer {{login.token}}
Content-Type: application/json

{
  "name": "Updated Name",
  "bio": "Hello, I'm {{$randomString(8)}}"
}

>>>
expect status 200
expect body.name == "Updated Name"
<<<

### List All Users (Admin)
# @name listUsers
# @tags admin
GET {{baseUrl}}/admin/users
Authorization: Bearer {{adminToken}}

>>>
expect status 200
expect body type array
expect body length > 0
expect body[0].id exists
expect body each type object
<<<
```

---

## See Also

- [CLI Reference](cli.md) - Command-line options
- [Environment Configuration](environments.md) - Environment setup
- [Examples](../examples/) - More example files
