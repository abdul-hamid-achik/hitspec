# hitspec.nvim

Neovim plugin for `.http` and `.hitspec` files. Provides syntax highlighting,
filetype detection, buffer settings, and LuaSnip snippets for the
[hitspec](https://github.com/abdul-hamid-achik/hitspec) HTTP API test runner.

## Features

- **Filetype detection** for `.http` and `.hitspec` files
- **Syntax highlighting** for HTTP methods, annotations, headers, variables,
  assertion blocks, capture blocks, built-in functions, and more
- **Buffer settings**: `commentstring`, 2-space indentation
- **LuaSnip snippets** for all HTTP methods, annotations, assertion/capture
  blocks, built-in functions, and complete request templates

## Installation

### lazy.nvim

```lua
{
  "abdul-hamid-achik/hitspec",
  config = function()
    require("hitspec").setup()
  end,
}
```

### packer.nvim

```lua
use {
  "abdul-hamid-achik/hitspec",
  config = function()
    require("hitspec").setup()
  end,
}
```

### Manual

Clone the repository into your Neovim packages directory:

```bash
git clone https://github.com/abdul-hamid-achik/hitspec.git \
  ~/.local/share/nvim/site/pack/hitspec/start/hitspec
```

Then add to your `init.lua`:

```lua
require("hitspec").setup()
```

## How It Works

- **Syntax highlighting** works automatically via the `syntax/hitspec.vim` file
  (no treesitter required).
- **Filetype detection** works automatically via `ftdetect/hitspec.lua`.
- **Buffer settings** are applied automatically via `ftplugin/hitspec.lua`.
- **LuaSnip snippets** require calling `require("hitspec").setup()` and having
  [LuaSnip](https://github.com/L3MON4D3/LuaSnip) installed. If LuaSnip is not
  available, `setup()` will skip snippet registration silently.

## Snippets

All snippets are available in `.http` and `.hitspec` buffers after calling `setup()`.

### HTTP Methods

| Trigger     | Description                         |
|-------------|-------------------------------------|
| `get`       | GET request template                |
| `post`      | POST request with JSON body         |
| `put`       | PUT request with JSON body          |
| `patch`     | PATCH request with JSON body        |
| `delete`    | DELETE request template             |
| `head`      | HEAD request template               |
| `options`   | OPTIONS request template            |
| `ws`        | WebSocket connection template       |

### Blocks

| Trigger      | Description                        |
|--------------|------------------------------------|
| `assert`     | Assertion block (`>>>` / `<<<`)    |
| `capture`    | Capture block (`>>>capture`)       |
| `graphql`    | GraphQL request with query block   |
| `db`         | Database assertion block           |
| `shell`      | Shell command block                |
| `multipart`  | Multipart form block               |
| `variables`  | Variables block                    |

### Annotations

| Trigger           | Description                    |
|-------------------|--------------------------------|
| `@name`           | Request name                   |
| `@description`    | Request description            |
| `@tags`           | Tags                           |
| `@timeout`        | Timeout in ms                  |
| `@retry`          | Retry count                    |
| `@retryOn`        | Retry on status codes          |
| `@retryDelay`     | Retry delay in ms              |
| `@if`             | Conditional execution          |
| `@unless`         | Inverse conditional execution  |
| `@depends`        | Dependency on other request    |
| `@auth`           | Auth config (choice node)      |
| `@skip`           | Skip request                   |
| `@only`           | Run only this request          |
| `@before`         | Pre-hook command               |
| `@after`          | Post-hook command              |
| `@db`             | Database connection string     |
| `@waitfor`        | Wait for URL health check      |
| `@import`         | Import another file            |
| `@stress.*`       | Stress test annotations        |
| `@contract.*`     | Contract test annotations      |

### Built-in Functions

| Trigger              | Output                          |
|----------------------|---------------------------------|
| `$uuid`              | `{{$uuid()}}`                   |
| `$timestamp`         | `{{$timestamp()}}`              |
| `$now`               | `{{$now(format)}}`              |
| `$random`            | `{{$random(min, max)}}`         |
| `$randomString`      | `{{$randomString(len)}}`        |
| `$randomEmail`       | `{{$randomEmail()}}`            |
| `$env`               | `{{$env(VAR)}}`                 |
| `$base64`            | `{{$base64(text)}}`             |
| `$md5`               | `{{$md5(text)}}`                |
| `$sha256`            | `{{$sha256(text)}}`             |
| `$urlEncode`         | `{{$urlEncode(text)}}`          |
| `$date`              | `{{$date(format)}}`             |

### Complete Template

| Trigger    | Description                                             |
|------------|---------------------------------------------------------|
| `request`  | Full request with name, tags, headers, body, assertions |

## Syntax Groups

The syntax file defines these highlight groups (linked to standard Vim groups):

| Group                  | Linked To    | Description              |
|------------------------|--------------|--------------------------|
| `hitspecMethod`        | `Keyword`    | HTTP methods             |
| `hitspecAnnotationKey` | `Keyword`    | Annotation names         |
| `hitspecComment`       | `Comment`    | Comments (`#`, `//`)     |
| `hitspecString`        | `String`     | Quoted strings           |
| `hitspecNumber`        | `Number`     | Numeric literals         |
| `hitspecVariable`      | `Special`    | `{{variable}}`           |
| `hitspecBuiltinFunc`   | `Function`   | `$func()` calls          |
| `hitspecHeaderKey`     | `Type`       | Header names             |
| `hitspecOperator`      | `Operator`   | Assertion operators      |
| `hitspecSeparator`     | `Title`      | `###` separators         |
| `hitspecURL`           | `Underlined` | Request URLs             |
