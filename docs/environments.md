# Environment Configuration

Configure different environments (dev, staging, prod) for your API tests.

## Overview

hitspec supports environment-specific variables through:

1. **Inline variables** in `.http` files
2. **Environment file** (`.hitspec.env.json`)
3. **System environment variables**

---

## Inline Variables

Define variables directly in your `.http` file:

```http
@baseUrl = https://api.example.com
@token = your-api-token
@timeout = 5000

GET {{baseUrl}}/users
Authorization: Bearer {{token}}
```

**Limitations:**
- Same values for all environments
- Secrets visible in test files
- Must change file to switch environments

---

## Environment File

Create `.hitspec.env.json` in your project root:

```json
{
  "dev": {
    "baseUrl": "http://localhost:3000",
    "token": "dev-token-123",
    "timeout": "10000"
  },
  "staging": {
    "baseUrl": "https://staging.api.example.com",
    "token": "staging-token-456",
    "timeout": "5000"
  },
  "prod": {
    "baseUrl": "https://api.example.com",
    "token": "prod-token-789",
    "timeout": "3000"
  }
}
```

### Usage

```bash
# Use dev environment (default)
hitspec run tests/

# Use staging environment
hitspec run tests/ --env staging

# Use production environment
hitspec run tests/ --env prod
```

### In Test Files

Reference environment variables the same way as inline variables:

```http
GET {{baseUrl}}/users
Authorization: Bearer {{token}}

>>>
expect duration < {{timeout}}
<<<
```

---

## Variable Resolution Order

Variables are resolved in this order (later overrides earlier):

1. **Built-in functions** (`$uuid()`, `$timestamp()`, etc.)
2. **Environment file** (`.hitspec.env.json`)
3. **Inline variables** (`@variable = value`)
4. **Captured values** (`{{requestName.captureName}}`)

---

## System Environment Variables

Reference system environment variables using `$env`:

```json
{
  "prod": {
    "baseUrl": "https://api.example.com",
    "token": "{{$env.API_TOKEN}}",
    "dbPassword": "{{$env.DB_PASSWORD}}"
  }
}
```

Or in `.http` files:

```http
@token = {{$env.API_TOKEN}}

GET {{baseUrl}}/users
Authorization: Bearer {{token}}
```

---

## Best Practices

### 1. Don't Commit Secrets

Add `.hitspec.env.json` to `.gitignore` if it contains secrets:

```gitignore
# .gitignore
.hitspec.env.json
```

Create a template file instead:

```json
// .hitspec.env.example.json
{
  "dev": {
    "baseUrl": "http://localhost:3000",
    "token": "YOUR_DEV_TOKEN_HERE"
  },
  "staging": {
    "baseUrl": "https://staging.api.example.com",
    "token": "YOUR_STAGING_TOKEN_HERE"
  }
}
```

### 2. Use System Variables for CI/CD

```json
{
  "ci": {
    "baseUrl": "{{$env.API_BASE_URL}}",
    "token": "{{$env.API_TOKEN}}"
  }
}
```

```yaml
# GitHub Actions
env:
  API_BASE_URL: https://staging.api.example.com
  API_TOKEN: ${{ secrets.API_TOKEN }}

steps:
  - run: hitspec run tests/ --env ci
```

### 3. Environment-Specific Test Behavior

Use different timeouts, retry settings, etc. per environment:

```json
{
  "dev": {
    "baseUrl": "http://localhost:3000",
    "timeout": "30000",
    "retries": "3"
  },
  "prod": {
    "baseUrl": "https://api.example.com",
    "timeout": "5000",
    "retries": "0"
  }
}
```

```http
# @timeout {{timeout}}
# @retry {{retries}}
GET {{baseUrl}}/slow-endpoint
```

### 4. Shared Base Configuration

Define common variables and override per environment:

```json
{
  "default": {
    "apiVersion": "v1",
    "timeout": "5000"
  },
  "dev": {
    "baseUrl": "http://localhost:3000"
  },
  "staging": {
    "baseUrl": "https://staging.api.example.com"
  },
  "prod": {
    "baseUrl": "https://api.example.com",
    "timeout": "3000"
  }
}
```

---

## Complete Example

### Project Structure

```
my-api-tests/
├── .hitspec.env.json
├── .hitspec.env.example.json
├── tests/
│   ├── auth.http
│   ├── users.http
│   └── orders.http
└── .gitignore
```

### .hitspec.env.json

```json
{
  "dev": {
    "baseUrl": "http://localhost:3000",
    "adminToken": "dev-admin-token",
    "userToken": "dev-user-token",
    "testEmail": "dev@example.com",
    "timeout": "10000"
  },
  "staging": {
    "baseUrl": "https://staging.api.example.com",
    "adminToken": "{{$env.STAGING_ADMIN_TOKEN}}",
    "userToken": "{{$env.STAGING_USER_TOKEN}}",
    "testEmail": "staging@example.com",
    "timeout": "5000"
  },
  "prod": {
    "baseUrl": "https://api.example.com",
    "adminToken": "{{$env.PROD_ADMIN_TOKEN}}",
    "userToken": "{{$env.PROD_USER_TOKEN}}",
    "testEmail": "prod-test@example.com",
    "timeout": "3000"
  }
}
```

### tests/auth.http

```http
### Login as Admin
# @name adminLogin
# @tags auth, admin
POST {{baseUrl}}/auth/login
Content-Type: application/json

{
  "email": "admin@example.com",
  "password": "{{$env.ADMIN_PASSWORD}}"
}

>>>
expect status 200
expect body.token exists
expect duration < {{timeout}}
<<<

>>>capture
adminToken from body.token
<<<

### Login as User
# @name userLogin
# @tags auth
POST {{baseUrl}}/auth/login
Content-Type: application/json

{
  "email": "{{testEmail}}",
  "password": "{{$env.USER_PASSWORD}}"
}

>>>
expect status 200
expect body.token exists
<<<

>>>capture
userToken from body.token
<<<
```

### Running Tests

```bash
# Development
hitspec run tests/ --env dev

# Staging (with environment variables)
STAGING_ADMIN_TOKEN=xxx STAGING_USER_TOKEN=yyy hitspec run tests/ --env staging

# CI/CD
hitspec run tests/ --env staging --output junit --output-file results.xml
```

---

## Troubleshooting

### Variable Not Found

```
Error: variable "baseUrl" not found
```

**Solutions:**
1. Check `.hitspec.env.json` has the variable for your environment
2. Verify you're using the correct `--env` flag
3. Check for typos in variable names

### Environment Not Found

```
Error: environment "staging" not found
```

**Solutions:**
1. Add the environment to `.hitspec.env.json`
2. Check for typos in environment name

### System Variable Not Set

```
Error: environment variable "API_TOKEN" not set
```

**Solutions:**
1. Export the variable: `export API_TOKEN=xxx`
2. Set it inline: `API_TOKEN=xxx hitspec run tests/`
3. Set it in CI/CD secrets

---

## See Also

- [Syntax Reference](README.md) - Complete .http file syntax
- [CLI Reference](cli.md) - Command-line options
