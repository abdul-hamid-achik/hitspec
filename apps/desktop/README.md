# hitspec Studio (desktop)

A native desktop client for [hitspec](../../README.md) — the file-based HTTP API
testing tool. It is a GUI over the same plain-text `.http` / `.hitspec` files
your CI runs: nothing is stored in a proprietary project format, and every edit
is written straight back to disk.

Under the hood the app spawns `hitspec serve --api-only` as a child process on a
random loopback port with a freshly generated bearer token, and drives it over
the REST + WebSocket API from `packages/serve`. The renderer never touches the
network or the token: every call crosses a small, explicit IPC bridge.

```
┌──────────────────────────── Electron ────────────────────────────┐
│  main (src/main)                                                  │
│    backend.js   spawns `hitspec serve --api-only`, owns the token │
│    ipc.js       REST proxy + native dialogs + guarded fs writes  │
│    menu.js      native menus / accelerators                      │
│    settings.js  recents, theme, window state (userData JSON)     │
│  preload (contextBridge, sandboxed)                               │
│  renderer (src/renderer, plain ES modules over a custom scheme)   │
│    views/  workspace · history · stress · mock · record ·         │
│            import · contracts · settings                          │
└───────────────────────────────────────────────────────────────────┘
            │  REST + WS (127.0.0.1:<random>, bearer token)
            ▼
      hitspec serve --api-only   (packages/serve)
```

## Features

- **Workspace browser** — file tree with request counts, fuzzy filter, request
  list per file, pass/fail dots from the last run.
- **Request inspector** — URL, headers, query params, body, assertions,
  captures, metadata (`@timeout`, `@retry`, `@depends`, `@auth`, `@skip`).
- **Runner** — run one request (`⌘↵`) or a whole file (`⌘⇧↵`) with live
  per-request progress from the WebSocket, then a tabbed response viewer
  (pretty-printed + highlighted JSON body, headers, assertion results, timing,
  captures, SSE events).
- **Source editor** — line numbers and `.http` syntax highlighting with no
  editor dependency; save (`⌘S`), revert, external-change reload via the file
  watcher.
- **Environments** — switch from the topbar, edit variables in a grid that
  writes back to `hitspec.yaml`.
- **History** — the persistent SQLite run history with per-run detail,
  assertions and body previews; delete single runs or clear all.
- **Stress** — load profiles (duration/rate/VUs), live RPS + p95 chart streamed
  over the WebSocket, per-request breakdown, thresholds, saved profiles.
- **Mock server** — start/stop, route table, live request log, copy base URL.
- **Recording proxy** — capture real traffic and export it as `.http` files.
- **Import** — curl, OpenAPI and Insomnia → preview → save into the workspace.
- **Contracts** — verify provider states against a live implementation.
- **Command palette** (`⌘K`), five themes shared with the terminal UI, toasts,
  native menus, and a welcome screen with recent workspaces.

## Development

Prerequisites: Node ≥ 20.11 and the Go toolchain used by the repo.

```bash
cd apps/desktop
npm install          # see the note below if the Electron binary is missing
task build           # from the repo root: builds bin/hitspec (or: npm run build:binary)
npm start            # launch the app
```

> npm ≥ 11 gates package install scripts. If `node_modules/electron/dist` is
> missing after install, run `node node_modules/electron/install.js` (or
> `npm install-scripts approve electron && npm rebuild electron`).

The app finds the CLI in this order: `$HITSPEC_BIN`, the bundled binary inside
`process.resourcesPath` (packaged builds), `apps/desktop/build/bin/hitspec`,
`<repo>/bin/hitspec`, then `hitspec` on `PATH`.

Open a workspace with `⌘O`, drop a folder on the Dock icon, or pass a directory
on the command line: `npm start -- /path/to/workspace`.

## Testing

```bash
npm run test:syntax   # parse every module (there is no bundler to catch typos)
npm test              # unit tests for the pure modules (node:test)
npm run test:smoke    # headless integration: real `hitspec serve` child + REST + WS
npm run test:e2e      # Playwright drives the real Electron app end to end
npm run check         # unit + smoke + e2e
```

The smoke and e2e suites build a throwaway workspace from
`test/fixtures/workspace/`, run an in-process HTTP target, and point the Go
server's `HOME` at a temp directory so your real `~/.hitspec/history.db` is
never touched.

## Packaging

```bash
npm run dist      # electron-builder (runs build:binary first via predist)
npm run dist:dir  # unpacked app, fastest way to sanity check a bundle
```

`electron-builder.yml` copies the Go binary into `extraResources`, so packaged
apps ship the CLI next to the Electron binary. macOS builds are unsigned by
default (`identity: null`); clear the quarantine flag after installing locally:
`xattr -dr com.apple.quarantine "/Applications/hitspec Studio.app"`.

## Keyboard shortcuts

| Keys | Action |
| --- | --- |
| `⌘/Ctrl + ↵` | Run the selected request |
| `⌘/Ctrl + ⇧ + ↵` | Run every request in the file |
| `⌘/Ctrl + S` / `⌘/Ctrl + ⇧ + S` | Save / revert the source editor |
| `⌘/Ctrl + O` / `⌘/Ctrl + N` | Open workspace / new file |
| `⌘/Ctrl + K` | Command palette |
| `⌘/Ctrl + ⇧ + R` | Restart the local API server |
| `⌘/Ctrl + 1…8` | Switch view |
| `e` | Focus the source editor |
| `?` / `F1` | Shortcuts / syntax reference |
| `Esc` | Close dialog or palette |

## Security model

- `contextIsolation: true`, `nodeIntegration: false`, `sandbox: true` preload.
- Renderer loads from a custom `hitspec://` scheme with a strict CSP; external
  navigation is denied and handed to the OS browser.
- The API token is generated per launch in the main process and never exposed
  to the renderer; the server binds to `127.0.0.1` only.
- `fs.writeFile` from the renderer only accepts paths the user picked in a
  native save dialog during the session; workspace paths are confined to the
  workspace root.
