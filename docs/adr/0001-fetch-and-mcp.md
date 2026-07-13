# ADR 0001: Response fetching and MCP integration

- Status: Accepted and implemented
- Date: 2026-07-13

## Context

Hitspec produced test reports and exported curl commands but had no command for
writing one response body to stdout or a file. It also had no bounded MCP
surface suitable for Local Agent and MCPHub. file.cheap already owns durable
artifact storage, so direct persistence from Hitspec would duplicate and couple
two product contracts.

## Decision

Add `hitspec fetch` as a separate single-request capability. Do not overload
`run --output`, whose values remain multi-request test reports, or
`export curl --exec`, which relies on an external shell and curl process.

The body representations are:

- `raw` (default): decoded response-body bytes with no metadata or newline;
- `text`: readable UTF-8, pretty JSON, or visible HTML text;
- `markdown`: a self-contained response document with sanitized provenance;
- `json`: an automation envelope with base64 for binary data.

CLI and MCP share `packages/fetch`. Saved requests reuse the parser,
environment resolver, config, auth, and Hitspec HTTP client. Every response is
bounded and limit failures are explicit. Output files are mode `0600`,
no-overwrite by default, symlink-resistant, and atomically placed.

Add `hitspec mcp serve --workspace PATH` using the official Go MCP SDK and
stdio. The initial tools are `hitspec_fetch`, `hitspec_list_requests`, and
`hitspec_validate`. stdout is protocol-only. Paths stay inside the fixed
workspace after symlink resolution; network access is public-only unless the
server operator grants `--allow-private-network`. Shell, database, history,
watching, and arbitrary output paths are absent.

Hitspec and file.cheap compose through an explicit file/CLI handoff. MCPHub
provides namespacing, lazy discovery, and temporary large-result spooling;
Local Agent uses the gateway without source changes. Hitspec does not import or
invoke file.cheap.

TinyVault integration is deferred. No secret values belong in files, command
arguments, MCP config, logs, provenance, or tool results. A later ADR must
define native references, precedence, redaction, and write-back behavior; until
then, process-environment injection remains outside Hitspec.

## Consequences

- Response download semantics no longer conflict with report formats.
- Binary data is safe across stdout, files, JSON, and MCP.
- CLI and MCP cancellation reaches in-flight HTTP requests.
- Agent-facing requests have explicit workspace, network, timeout, and size
  authority.
- Durable persistence remains a composable file.cheap operation rather than a
  hidden side effect of fetching.

## Rejected alternatives

- Add raw/text/Markdown to `run --output`: ambiguous for multiple requests.
- Extend `export curl --exec`: external process, weak cancellation, no typed
  reusable result.
- Save directly into file.cheap: tight coupling and a hidden persistence side
  effect.
- Accept arbitrary MCP output paths: unnecessary filesystem authority.
