# Studio end-to-end tests

Black-box behavior tests for `hitspec studio`, driven through a real pseudo-terminal
by [glyphrun](https://github.com/abdul-hamid-achik/glyphrun) (the `glyph` binary).

Each spec launches the studio binary in a PTY, drives it with keystrokes, and
asserts against the rendered virtual-terminal screen and the process exit code —
no coupling to studio's internals.

## Running

```bash
task e2e                                   # build + run the whole suite
# or directly (from the repo root):
go build -o ./e2e/bin/hitspec ./apps/cli
glyph run e2e/specs/glyphrun/*.yml --format md
```

Requires `glyph` on PATH (`brew install abdul-hamid-achik/tap/glyph` or
`go install github.com/abdul-hamid-achik/glyphrun/cmd/glyph@latest`).

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
| `.glyphrun/runs/` | artifact packs per run (gitignored) |

## Specs

| Spec | What it proves |
|------|----------------|
| `smoke.yml` | launches on a workspace; shows branding, file list, nav strip; no "TUI" jargon; clean quit |
| `welcome.yml` | empty workspace shows the first-run welcome card |
| `generate_sample.yml` | `g` scaffolds `example.http` + `hitspec.yaml` (asserted on screen **and** on disk) |
| `help_overlay.yml` | `?` opens the keyboard help overlay; any key dismisses it |
| `navigation.yml` | number keys switch screens (workspace → stress → history → workspace) |
| `theme_flag.yml` | `--theme` launches with an alternate color theme without error |

## Note on key encoding

These specs drive studio with printable keys (`q`, `g`, digits) and `ctrl+<key>`
(`ctrl+t`). The Bubble Tea v2 input parser does **not** match the `enter`/`esc`
key *bindings* when those keys arrive as bare control bytes over glyphrun's PTY
(real terminals and the Go unit tests in `packages/tui` exercise those paths
directly). So overlays that close only on Enter/Escape (theme picker, env
switcher, command-palette execute) are covered by `packages/tui` unit tests and,
for theming, by the `--theme` flag spec here rather than by driving the picker.
