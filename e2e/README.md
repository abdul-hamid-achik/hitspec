# Studio end-to-end tests

Black-box behavior tests for `hitspec studio`, driven through a real pseudo-terminal
by [glyphrun](https://github.com/abdul-hamid-achik/glyphrun) (the `glyph` binary).

Each spec launches the studio binary in a PTY, drives it with keystrokes, and
asserts against the rendered virtual-terminal screen and the process exit code —
no coupling to studio's internals.

(The `hitspec serve` REST API has its own Go tests in `packages/serve`; this
folder is the terminal-UI suite.)

## Running

```bash
task e2e                 # build + run the whole suite
task e2e -- settings     # run one spec (bare name from flows/, or a path)
task --watch e2e         # rebuild + rerun on any TUI or spec change

# or directly (from the repo root):
go build -o ./e2e/bin/hitspec ./apps/cli
glyph run e2e/flows/*.yml --format md
```

Requires `glyph` on PATH (`brew install abdul-hamid-achik/tap/glyph` or
`go install github.com/abdul-hamid-achik/glyphrun/cmd/glyph@latest`).

Glyphrun discovers `glyphrun.config.yml` by walking up from each spec, and
resolves spec paths (`cwd`, preconditions, `artifactRoot`, `vars`) **relative to
that config's directory** (this `e2e/` folder). Each spec rebuilds the binary as
a precondition, so the specs are self-contained.

## Layout

Flows, actions, and fixtures live directly under `e2e/`, alongside the config
and the build/artifact dirs:

| Path | Purpose |
|------|---------|
| `glyphrun.config.yml` | terminal defaults, vars (`bin`, `workspace`, …), env |
| `flows/*.yml` | the behavior specs |
| `actions/*.yml` | reusable action snippets (imported, invoked via `use:`) |
| `fixtures/workspace/` | a sample `.http` workspace |
| `fixtures/empty/` | an empty workspace (triggers the welcome card) |
| `bin/` | the built `hitspec` binary (gitignored) |
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
| `workspace_layout.yml` | workspace renders bordered panels with a bounded files sidebar (no spill) |
| `settings.yml` | settings shows the editable form **and** the config dump together |
| `form_edit.yml` | `e` focuses a secondary-screen form field; typed digits land in it (no screen jump) |
| `form_navigate.yml` | `down` moves focus to the next form field; typing fills whichever field is focused |
| `quit_from_overlay.yml` | `ctrl+c` quits cleanly even with the command palette open (hard interrupt) |
| `search_overlay.yml` | `ctrl+f` opens search; typing filters requests; `ctrl+c` closes and exits |
| `all_modules.yml` | every numbered screen (stress/mock/contract/record/import/cookies) renders its module |
| `theme_picker.yml` | `ctrl+t` opens the theme picker overlay; `ctrl+c` closes and exits |
| `env_switcher.yml` | `ctrl+e` opens the environment switcher overlay; `ctrl+c` closes and exits |
| `run_request.yml` | `R` runs a file (closed local port → deterministic fail); response pane + tabs reachable |
| `edit_save.yml` | `e` edits the source, text marks it modified, `ctrl+s` saves and reloads |
| `focus_cycle.yml` | `tab` cycles files → requests → source → response; hints update per pane |
| `history_after_run.yml` | a run is persisted and appears on the history screen (`6`) |
| `adhoc_request.yml` | palette → filter "Quick" → prompt a URL → run a one-off ad-hoc request |
| `duplicate_file.yml` | palette → filter "Duplicate" → accept prompt → the copy appears (scratch ws) |
| `rename_file.yml` | palette → filter "Rename" → clear + retype the path → file renamed (scratch ws) |
| `settings_save.yml` | settings form: `e` edit → set retries → submit → config dump updates (scratch ws) |
| `response_tabs.yml` | after a run, `]` cycles the response viewer tabs (Body…Captures) |
| `import_scratch.yml` | import screen submits the curl form → generated request opens (scratch ws) |
| `clear_history.yml` | palette → "Clear history" → confirm `y` → history list empties (isolated HOME) |
| `mock_start_stop.yml` | mock screen: `s` starts the server (custom port), `x` stops it (scratch ws) |
| `stress_run.yml` | stress screen: edit duration to 2s, start, watch live metrics, see the result |
| `record_start_stop.yml` | record screen: `s` starts the proxy (custom port), `x` stops it |

(The palette "copy as curl" export is covered by a `packages/tui` unit test
rather than a spec — its clipboard subprocess made the e2e assertion flaky under
full-suite load.)

Specs that run requests or touch cookies set `target.env.HOME` to a throwaway dir
so they never read or pollute the user's real `~/.hitspec` history/cookie store
(`target.env` applies to the studio process only — preconditions still build with
the real `HOME`).

Reusable actions live in `actions/` (`quit_clean`, `open_workspace`) and
are pulled in with `imports:` + `use:`.

## Note on key encoding

These specs drive studio with printable keys (`q`, `g`, digits), `ctrl+<key>`
(`ctrl+t`, `ctrl+p`, `ctrl+f`), **and `enter`/`esc`** — the latter now bind
correctly over glyphrun's PTY (earlier toolchain versions didn't, so older specs
leaned on `ctrl+c` to quit and the `--theme` flag instead of driving the picker;
those still work and are kept). `enter`/`esc` are exercised directly now by the
palette/prompt specs (`adhoc_request`, `duplicate_file`, `copy_as_curl`).

Palette tip: the command list filters via `/` then a query (not type-to-filter).
Use a **short, unique** token and `wait` for `1 item` before `enter`, both to
avoid dropped keystrokes on long queries and so `enter` executes the single match
rather than just applying the filter.
