# Studio end-to-end tests

Two complementary black-box suites:

- **glyphrun** (`specs/glyphrun/`) — drives `hitspec studio` (the terminal UI)
  through a real pseudo-terminal, asserting against the rendered virtual-terminal
  screen and the process exit code. This is the UI suite.
- **cairntrace** (`cairntrace/`) — drives the `hitspec serve` REST API
  (`/api/v1/*`, the same functional layer the TUI consumes) through a browser
  backend, asserting JSON responses and request health. This is the API suite.

Each glyphrun spec launches the studio binary in a PTY, drives it with
keystrokes, and asserts against the screen — no coupling to studio's internals.

## Running

```bash
task e2e            # glyphrun TUI suite (build + run)
task e2e-cairn      # cairntrace serve-API suite (starts/stops the server for you)
task e2e-all        # both suites

# or directly (from the repo root):
go build -o ./e2e/bin/hitspec ./apps/cli
glyph run e2e/specs/glyphrun/*.yml --format md
```

Requires `glyph` on PATH (`brew install abdul-hamid-achik/tap/glyph` or
`go install github.com/abdul-hamid-achik/glyphrun/cmd/glyph@latest`) for the TUI
suite, and `cairn` (cairntrace) + `agent-browser` for the API suite.

### cairntrace serve-API suite

`task e2e-cairn` builds the binary, starts `hitspec serve --api-only` on port
4517 against a scratch workspace, waits for readiness, runs the flows in
`cairntrace/flows/`, and always stops the server. Run flows manually with the
server already up:

```bash
hitspec serve --api-only --cors --port 4517 /tmp/hitspec-cairn-ws &
cairn run e2e/cairntrace/flows --format md
```

Flows cover system info, workspace + file CRUD, environments + config, run
execution, curl import, and the stress/mock/record status endpoints. They use
`httpJson`/`noFailedRequests` verifiers and a `script` escape hatch; the
`execute_run` flow targets the server's own endpoint so it needs no external
network. Run artifacts land in `~/.cairntrace/runs` (outside the repo).

Glyphrun discovers `glyphrun.config.yml` by walking up from each spec, and
resolves spec paths (`cwd`, preconditions, `artifactRoot`, `vars`) **relative to
that config's directory** (this `e2e/` folder). Each spec rebuilds the binary as
a precondition, so the specs are self-contained.

## Layout

| Path | Purpose |
|------|---------|
| `glyphrun.config.yml` | terminal defaults, vars (`bin`, `workspace`, …), env |
| `specs/glyphrun/*.yml` | the behavior specs |
| `fixtures/workspace/` | a sample `.http` workspace |
| `fixtures/empty/` | an empty workspace (triggers the welcome card) |
| `.glyphrun/runs/` | glyphrun artifact packs per run (gitignored) |
| `cairntrace/cairntrace.config.yml` | cairntrace config (baseUrl → the serve API) |
| `cairntrace/flows/*.yml` | serve-API behavior flows |

## Specs

| Spec | What it proves |
|------|----------------|
| `smoke.yml` | launches on a workspace; shows branding, file list, nav strip; no "TUI" jargon; clean quit |
| `welcome.yml` | empty workspace shows the first-run welcome card |
| `generate_sample.yml` | `g` scaffolds `example.http` + `hitspec.yaml` (asserted on screen **and** on disk) |
| `help_overlay.yml` | `?` opens the keyboard help overlay; any key dismisses it |
| `navigation.yml` | number keys switch screens (workspace → stress → history → workspace) |
| `theme_flag.yml` | `--theme` launches with an alternate color theme without error |
| `workspace_layout.yml` | workspace renders bordered panels with a bounded files sidebar (no spill) |
| `settings.yml` | settings shows the editable form **and** the config dump together |
| `form_edit.yml` | `e` focuses a secondary-screen form field; typed digits land in it (no screen jump) |
| `form_navigate.yml` | `down` moves focus to the next form field; typing fills whichever field is focused |
| `quit_from_overlay.yml` | `ctrl+c` quits cleanly even with the command palette open (hard interrupt) |
| `search_overlay.yml` | `ctrl+f` opens search; typing filters requests; `ctrl+c` closes and exits |
| `all_modules.yml` | every numbered screen (stress/mock/contract/record/import/cookies) renders its module |
| `theme_picker.yml` | `ctrl+t` opens the theme picker overlay; `ctrl+c` closes and exits |
| `env_switcher.yml` | `ctrl+e` opens the environment switcher overlay; `ctrl+c` closes and exits |

## Note on key encoding

These specs drive studio with printable keys (`q`, `g`, digits) and `ctrl+<key>`
(`ctrl+t`). The Bubble Tea v2 input parser does **not** match the `enter`/`esc`
key *bindings* when those keys arrive as bare control bytes over glyphrun's PTY
(real terminals and the Go unit tests in `packages/tui` exercise those paths
directly). So overlays that close only on Enter/Escape (theme picker, env
switcher, command-palette execute) are covered by `packages/tui` unit tests and,
for theming, by the `--theme` flag spec here rather than by driving the picker.
