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
npm run dist       # installers for this OS (runs build:binary --dist first via predist)
npm run dist:dir   # unpacked app(s), fastest way to sanity check a bundle
npm run test:bundled   # each unpacked bundle carries a CLI for its own OS and CPU
npm run test:fuses     # the release fuses are flipped (see below)
npm run build:binary:all   # cross-compile the CLI for every target
```

`scripts/build-binary.mjs` cross-compiles the (pure Go, `CGO_ENABLED=0`) CLI
into `build/bin/<os>-<arch>/` (`mac-arm64`, `mac-x64`, `win-x64`, `linux-x64`).
`electron-builder.yml` copies `build/bin/${os}-${arch}` into `extraResources`
per target, so an x64 build can never carry the arm64 CLI and the Windows build
gets `hitspec.exe`. In development the app looks in
`build/bin/<os>-<arch>/` for the running CPU first.

Electron fuses are flipped by electron-builder before signing (`electronFuses`
in `electron-builder.yml`): `RunAsNode`, `NODE_OPTIONS` and `--inspect` are off;
ASAR integrity validation and `OnlyLoadAppFromAsar` are on.
`resetAdHocDarwinSignature` is required: without it an unsigned arm64 macOS app
is SIGKILLed at launch because flipping fuses invalidates Electron's ad-hoc
signature. Playwright attaches through the inspector, so `npm run test:packaged`
boots a *test-only* build (`npm run dist:dir:test` -> `release-test/`, inspect
fuse back on); never ship that output.

Signing is environment driven: without `CSC_LINK` (or a Developer ID certificate
in the keychain) builds are unsigned; `CSC_IDENTITY_AUTO_DISCOVERY=false` forces
that. Notarization is `-c.mac.notarize=true` with `APPLE_ID`,
`APPLE_APP_SPECIFIC_PASSWORD` and `APPLE_TEAM_ID`. Unsigned macOS builds need
`xattr -dr com.apple.quarantine "/Applications/hitspec Studio.app"`.

CI is `.github/workflows/desktop.yml`: the four test suites on Linux and macOS,
a per-OS packaging job (`electron-builder --dir`, bundled-CLI and fuse checks),
and an `installers` job (manual run or `desktop-v*` tag) that signs, notarizes
and publishes only when the credentials exist in the repository secrets.

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
  to the renderer; it reaches the Go child through the `HITSPEC_API_TOKEN`
  environment variable (never argv, which `ps` shows), and the server binds to
  `127.0.0.1` only. The server also enforces a Host allowlist, rejects foreign
  `Origin` headers and requires `Content-Type: application/json` on JSON
  bodies; the main process's `fetch`/`ws` calls satisfy all three
  (`test/unit/backend.test.js` pins this).
- `fs.writeFile` from the renderer only accepts paths the user picked in a
  native save dialog during the session; workspace paths are confined to the
  workspace root.
- Every IPC handler checks `event.senderFrame`: only the top-level
  `hitspec://app/` page of one of our windows is served (`src/main/policy.js`).
- `api(req)` is not a passthrough. Method + path must match an allowlist that
  mirrors `src/renderer/src/api.js` (a unit test fails when they drift); paths
  with `..`, encoded separators, query strings or foreign hosts are rejected,
  and only the known request fields are forwarded.
- The renderer cannot write `backend.childEnv` or `backend.allowShell`
  (`sanitizeSettingsPatch`), and the settings file is scrubbed of both on load.
  The child's environment never contains `DYLD_*`, `LD_*`, `NODE_OPTIONS`,
  `ELECTRON_*` or `PATH` overrides. `--allow-shell` needs a native confirmation
  dialog per workspace; the approval is in memory only and lasts until quit.
- CSP is set both as a `<meta>` tag and as a header on every `hitspec://`
  response (no `unsafe-inline`, `frame-ancestors 'none'`); permission requests
  (camera, geolocation, ...) are denied; `<webview>` is disabled.
- Release builds flip the Electron fuses (see Packaging).

## Lifecycle

- **Spawn failures** (`EACCES`, wrong CPU architecture, macOS Gatekeeper
  killing an unsigned binary) reject `start()` immediately with a message that
  names the binary and a fix, instead of waiting out the 30 s readiness timeout.
- **Version gate:** the app reads `version` from `GET /api/v1/system/info` and
  warns (in a toast and in Settings) when the CLI is older than v2.20.0. Source
  builds (`dev`, bare commit hashes) are assumed current.
- **Unsaved edits** are guarded when switching files, closing the window,
  quitting, reloading and opening another workspace (native dialog in the main
  process, fed by the renderer's dirty flag). Auto-reload from the file watcher
  never overwrites a dirty buffer, and saving re-reads the file first: if its
  content differs from what the editor last saw, the user is asked before
  overwriting.
- **Large responses:** bodies above 512 KB are shown as plain text with a
  "Highlight" button, the pretty-printed body is memoised, and stress/mock
  events no longer re-render views that are not on screen.

### Orphaned API server after SIGKILL

If Electron dies without running any code (SIGKILL, kernel OOM kill, power
loss), the `hitspec serve --api-only` child would keep its port and file
watcher. Two layers handle it:

- the desktop starts the server with `--parent-pid <electron pid>`; the server
  polls that process every second and shuts down gracefully when it is gone. A
  CLI older than the flag is detected (`unknown flag: --parent-pid`) and started
  without it;
- the child's pid and port are written to `<userData>/serve.pid.json`, and the
  next launch stops a leftover server after verifying its full command line
  (`hitspec serve --api-only ... --port <recorded port>`, read with `ps` or, on
  Windows, `Get-CimInstance Win32_Process`). A reused pid that now belongs to
  another program, or to another hitspec command, is never signalled.

Graceful quit, `SIGINT`/`SIGTERM`/`SIGHUP` and `process.on('exit')` still stop
the child directly.

Launching the app again with a folder (`hitspec-studio <dir>`, or macOS "Open
With") opens it in the running window through the same guarded path as File >
Open Workspace.
