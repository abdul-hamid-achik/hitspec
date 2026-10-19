# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

## [2.24.0] - 2026-10-19

`reporters` and `outputDir` in `hitspec.yaml` now take effect in `hitspec run`:
console on stdout plus JUnit, JSON, TAP or HTML files for CI, without flags.
The studio stress screen runs saved stress profiles, and workspace search no
longer re-parses every file on each keystroke. On Windows, a connection reset
before any response is a network error (exit 4) as on other platforms. **Read
the upgrade notes below before you bump hitspec in CI.**

### Upgrade notes: tests that may start failing

Nothing changes for a `hitspec.yaml` without `reporters` and `outputDir`, or
with the defaults (`reporters: [console]`, no `outputDir`). Check the items
below if yours sets either key: they used to be read and ignored.

**`reporters` and `outputDir` apply in `hitspec run`**

- **A `reporters` entry that is not `console`, `json`, `junit`, `tap` or
  `html` exits 3** with `Error: hitspec.yaml: unknown reporter "xml" (valid:
  console, json, junit, tap, html)`. It used to be ignored. Fix the name, or
  pass `-o` (which skips the keys).
- **Two or more reporters without `outputDir` exit 3**: without `outputDir`
  every reporter writes to stdout, and only one can. Set `outputDir`, or keep
  one reporter.
- **One non-console reporter without `outputDir` replaces the console report
  on stdout.** `reporters: [json]` now behaves like `-o json`. A script that
  reads the console report of such a project gets JSON.
- **With `outputDir`, report files are written there**:
  `hitspec-results.json`, `hitspec-junit.xml`, `hitspec-results.tap` and
  `hitspec-report.html`, one per listed reporter, replaced on every run. A
  relative `outputDir` is relative to the config file. hitspec creates it
  when a report goes there; if it cannot, the run exits 3. Add it to
  `.gitignore` if needed.
- **When no report goes to stdout** (for example `reporters: [junit]` with
  `outputDir: reports`), failed requests are listed on stderr as
  `FAIL file: name: reason`, as with `--quiet`. Exit codes are unchanged.
- `-o`/`--output`, `--output-file`, or a non-empty `HITSPEC_OUTPUT` or
  `HITSPEC_OUTPUT_FILE` still decide alone: the yaml keys are then not even
  read. `--stress`, `--dry-run`, `validate`, `studio` and `serve` ignore them.

**Windows**

- **A connection the server resets before any response exits 4** (network
  error), as on Linux and macOS. It used to exit 1 when Winsock reported the
  reset (`wsasend: An existing connection was forcibly closed by the remote
  host`), for example a server that rejects the client certificate. A CI step
  that expected exit 1 there must expect 4.

**hitspec studio**

- The stress form has a new last field, `profile`. The other fields keep their
  order, so `e` and typing a duration work as before. Scripts that tab through
  the form see one more field. The placeholders read `30s or profile's`,
  `5 or profile's`, and so on.
- Workspace search re-parses a file only when its modification time or size
  changed or studio wrote it. An edit by another tool that keeps both the same
  shows up after the next change to the file.

### Added

- `reporters` and `outputDir` in `hitspec.yaml` (see the upgrade notes and
  [Configuration](https://hitspec.dev/reference/configuration#reporters-and-outputdir)).
- hitspec studio: type a saved stress profile's name in the stress form's
  `profile` field. Its duration, rate, VUs and thresholds apply; values typed
  in the form win. The idle stress screen lists the saved profiles.

### Fixed

- On Windows, a Winsock connection reset or abort before any response counts
  as a network error.
- hitspec studio: an error for a search query the user has already typed past
  no longer shows a toast. Search honors cancellation between files.
- The stress-testing guide said the studio TUI edits profiles; it runs them by
  name, and the desktop app edits them.

### Changed

- hitspec studio: workspace search caches parsed files, so typing in the
  search overlay no longer re-parses the whole workspace.
- The macOS test job is required in CI, like Windows (it passed 60 runs in a
  row first).

## [2.23.2] - 2026-10-19

Fixes a 2.23.1 regression: `hitspec run --stress` printed color codes into
files and pipes. **The upgrade notes below list what a CI job can notice.**

### Upgrade notes: tests that may start failing

- **`hitspec run --stress` output has no color codes when stdout is not a
  terminal.** 2.23.1 forced the stress reporter's colors on, so
  `hitspec run --stress > stress.txt` and CI logs got ANSI escape codes. They
  are gone again, as in 2.23.0 and before. Only a check that expected those
  codes in a file or pipe is affected. `NO_COLOR` and `TERM=dumb` turn the
  colors off again too.
- **Go library: `output.WithNoColor(true)` no longer sets the global
  `color.NoColor`.** It turns colors off for that console formatter only.
  A program that relied on it to turn off every other `fatih/color` output
  must set `color.NoColor` itself.

### Fixed

- `hitspec run --stress` follows the terminal detection again: no colors in
  files, pipes, `NO_COLOR` or `TERM=dumb`.
- `hitspec run --watch` turns colors back on when `noColor` changes from
  `true` to `false` in `hitspec.yaml`. Once off, they used to stay off for the
  rest of the session.
- The console report no longer writes a process-wide color setting, so two
  formatters at once do not race on it.

## [2.23.1] - 2026-10-19

Small fixes on top of 2.23.0:
- `--metrics` without `--stress` warns instead of doing nothing;
- stress metrics report status codes;
- `hitspec export --exec` no longer needs bash;
- studio and `serve` keep `__snapshots__` inside the workspace.

**The upgrade notes below list what a CI job can notice.**

### Upgrade notes: tests that may start failing

- **`--metrics` without `--stress` prints a warning.** `hitspec run --metrics
  json` (or `HITSPEC_METRICS` set in the environment) on a normal run used to
  export nothing without a word. It still exits 0 and exports nothing, but
  stderr now says `warning: --metrics only applies with --stress; ...`.
  A CI step that fails on any stderr output, or a `HITSPEC_METRICS` exported
  for every job, will see it. Add `--stress`, or unset the variable for
  non-stress runs.
- **studio and `serve` refuse a `__snapshots__` directory or snapshot file
  that a symlink points to outside the workspace.** The snapshot assertion
  fails with `snapshot: file reference "<path>" is outside the workspace`.
  It used to read and, in update mode, write there. `hitspec run` on the
  command line is unchanged. Keep snapshots inside the workspace.
- **`status_codes` is filled in the stress metrics.** This covers the JSON
  file, Prometheus's status-code series and DataDog. It used to be empty
  (`{}`), because the stress summary never counted status codes. Dashboards
  and checks that expected it empty will now see data. Requests that got no
  response (timeouts, connection errors) are not counted there.
- **`hitspec export --exec` runs the generated curl command through `sh`**
  instead of `bash`. On Windows that is Git for Windows' `sh`, or `cmd /C`
  when it is not installed. The command is plain POSIX quoting, so nothing
  changes for it. Systems without bash can now use `--exec`. curl must still
  be installed.

### Fixed

- `hitspec serve` no longer prints the stress progress and summary to its
  stdout. The desktop app runs it as a child process; use the REST or
  WebSocket result instead.
- Two stress runs at once in `serve` no longer race on the global color
  setting.
- `hitspec run --watch` picks up a changed `noColor` in `hitspec.yaml` on the
  next pass.
- The run reference and the metrics guide say that `--metrics` applies to
  `--stress` runs.

## [2.23.0] - 2026-10-19

hitspec's whole test suite now runs on Windows, and this release fixes what
that turned up. Hooks and `>>>shell` blocks run through Git for Windows' `sh`,
quoted globs work, and studio and `serve` report paths with `/`. Two
`hitspec.yaml` keys now take effect: `noColor`, and `verbose`, which also
prints the config line. A busy port is a configuration error (exit 3) in
every command. Stress runs started from a saved profile in studio, `serve` or
the desktop app use the profile's thresholds, and the run history keeps
warnings. **Read the upgrade notes below before you bump hitspec in CI.**

### Upgrade notes: tests that may start failing

Most of this release fixes bugs and adds fields. Check the items below if your
CI matches exit codes or error text, runs on Windows, or reads the `serve`
JSON with a strict schema. As before: run `hitspec validate <dir>` and the
suite once locally with the new version, and pin the version in CI.

**Exit codes**

- `hitspec mock` and `hitspec record` exit **3** instead of 1 when their
  port is already in use. That is the configuration-error code, so a busy
  port now reads the same in every command.
- `hitspec run --stress --metrics prometheus` with a `--metrics-port` that is
  already in use exits **3** before any request is sent. It used to print the
  error and run the whole load test without a metrics endpoint. Free the port
  or pick another with `--metrics-port`.

**Settings that now apply**

- `noColor: true` in `hitspec.yaml` turns colors off in `hitspec run`, in the
  console report and the stress reporter. It used to be read and then
  ignored. `--no-color=false` or `HITSPEC_NO_COLOR=false` turns colors back
  on. `NO_COLOR` in the environment still turns them off.
- With `verbose: true` in `hitspec.yaml`, stderr now starts with the same
  `config: <path>` line that `-v` prints. `--verbose=0` turns both off. A
  test that compares stderr exactly will see the extra line.

**Files outside the workspace**

- In studio, `serve` and the desktop app, `expect body schema <file>` refuses
  a schema that a symlink points to outside the workspace. The assertion
  fails with `outside the workspace`; it used to read the file. This applies
  to normal runs and to stress and contract runs. `../` and absolute paths
  were already refused. `hitspec run` on the command line is unchanged. If
  you hit this, move the schema into the workspace.

**Stress runs from a saved profile**

- In the desktop app, **Apply** on a saved stress profile now sends the
  profile with the run, so its thresholds (`rps`, `p95`, `errors`, ...) are
  checked. A run that used to "pass" because thresholds were ignored can now
  report failed thresholds. The values in the form still win over the
  profile's own; **Clear** next to the applied profile drops it.

**Windows**

- Hooks (`# @before`, `# @after`) and `>>>shell` blocks run through `sh` from
  Git for Windows when it is installed (on `PATH` or in the default install).
  Without it they fall back to `cmd /C`, where POSIX shell syntax does not
  work. They used to need `sh` on `PATH` and failed without it.
- A contract `--state-handler` that is not an `.exe`, `.bat`, `.cmd` or `.com`
  file, such as `setup-states.sh`, runs through that `sh`. It used to fail
  with "not a valid Win32 application".
- Quoted globs such as `hitspec run 'tests/**/*.http'` work. They used to stop
  with `cannot access`.
- studio, `serve` (JSON and WebSocket events) and the desktop app report
  workspace-relative paths with `/` (`api/users.http`), and accept `/` or
  `\` on input. Clients that compared against `api\users.http` must use `/`.
  `hitspec import ... -o` prints `/` paths too.
- The MCP server refuses `/x`, `\x` and `C:x` as paths, the way it refuses
  absolute paths elsewhere. It used to treat them as workspace-relative.

**Error text on every platform**

- Errors about a CA bundle, client certificate or key put the path in plain
  quotes: `"C:\certs\ca.pem"`, not `"C:\\certs\\ca.pem"`. Only tests that
  match that error text exactly are affected.

**JSON from `serve` and studio**

- New fields, all omitted when empty:
  - `duration` on `GET /api/v1/stress/status` and on `stress_update` events
    (the run's configured length, in seconds);
  - `warnings` on each result in `GET /api/v1/history/runs/{id}`;
  - an optional `profile` on `POST /api/v1/stress/start`, which makes
    `duration` optional.

  A client that validates responses against a strict schema must allow
  these fields. The history database gets a `result_warnings` table the
  first time 2.23.0 opens it. Older hitspec versions keep working with it.

**hitspec studio**

- `enter` in the search overlay selects the request it found. It switches to
  the workspace (opening the file if needed) and focuses the request list. A
  hit in the open file moves the cursor without asking to discard edits.
- The stress progress bar follows the run's own duration. When that is
  unknown, it shows the elapsed time without a percentage.
- When the open file is deleted outside studio, unsaved edits are kept and a
  warning says so. They used to be cleared silently. `ctrl+s` recreates the
  file.

**Go library users**

- `metrics.NewPrometheusExporter` no longer prints a bind error to stdout. On
  a busy port it returns an exporter with no endpoint. Use
  `metrics.NewPrometheusExporterE` to get the error.
- A context from `clientmgr.WithExecID` sets the `ExecID` on that run's
  events.

### Security

- studio, `serve` and the desktop app resolve symlinks before reading a
  `schema` file, so a link inside the workspace cannot reach a file outside
  it.
- On Windows, the MCP server refuses rooted and drive-relative paths instead
  of re-rooting them inside the workspace.

### Added

- The run history keeps parser and runtime warnings for each result. studio
  and `serve` return them in the run details.
- `POST /api/v1/stress/start` (and `Manager.StartStress`) takes a saved
  `profile`: its duration, rate, VUs and thresholds are the base, and the
  values in the request win. The desktop app sends the applied profile.
- `duration` on the stress status and `stress_update` events. Clients can
  draw progress as `elapsed / duration`.
- Library: `stress.ApplyProfile`, `stress.ThresholdString`,
  `metrics.NewPrometheusExporterE`, `clientmgr.WithExecID`,
  `assertions.WithFileRoot` and `http.CheckPathWithinRoot`.

### Fixed

- `noColor` and `verbose` in `hitspec.yaml` (see the upgrade notes).
- The Prometheus endpoint is bound before the run starts. Its server field is
  no longer read and written without a lock.
- Cancelling an OpenAPI import from studio, or a client disconnecting from
  `POST /api/v1/import/openapi`, stops the fetch.
- A build from a checkout with no tags (a shallow CI clone, a fork) reports
  `dev+<revision>` in `hitspec version` and `/api/v1/system/info`. It used to
  report `2.0.0-<time>-<revision>`, and hitspec Studio then warned that the CLI
  was older than 2.20.0.
- On Windows:
  - hooks, `>>>shell` blocks, state handlers and globs work;
  - paths in studio and `serve` use `/`;
  - the studio cookie screen shows `~/.hitspec/tui-cookies.json`;
  - checkouts keep LF line endings (`.gitattributes`).
- hitspec studio:
  - search selects its result;
  - stress progress uses the run's duration;
  - a run ignores progress events from other executions;
  - a file deleted outside studio keeps its unsaved edits.

### Changed

- The Windows test job is required in CI. The suite runs on Linux, macOS and
  Windows.

## [2.22.0] - 2026-10-19

`hitspec run` explains its failures, tells a network problem apart from a
wrong answer, and finds `hitspec.yaml` the way git finds `.git`. Requests are
sent the way you wrote them: query parameters keep their order, form values
keep their spaces, and a missing body file is an error, not a literal string.
mock, record and the metrics endpoint listen on loopback by default. Some CI
setups need a small change: **read the upgrade notes below before you bump
hitspec in CI.**

### Upgrade notes: tests that may start failing

Most of this release adds things, but some changes show up as a different exit
code, a request sent differently, or a stricter check. To find them before
your CI does:

1. Run `hitspec validate <dir>` with the new version. It now also warns about
   unknown `hitspec.yaml` keys (`--strict` makes them errors).
2. Run the suite once locally and compare the exit code with what your CI
   step expects: 4, 64 and 130 can appear where you used to get 1, and 3 is
   used in more places.
3. Pin the version in CI (the GitHub Action's `version:` input, or
   `go install .../cmd/hitspec@v2.22.0`) and bump it on purpose.

**Exit codes**

- A request that gets no response because the connection failed now makes
  `hitspec run` exit **4** instead of 1: connection refused, DNS failures, TLS
  and certificate errors (including a server that rejects your client
  certificate), a connection reset before any response, a timeout before the
  connection was made, an OAuth2 token endpoint that cannot be reached, and a
  `@waitFor` target that cannot be reached. A CI step that only accepts 0 or 1
  must treat 4 as a failure too. A timeout after connecting, a body that breaks
  half-way and assertion failures are still 1. When several apply, the order
  is 130, 2, 4, 1, 64.
- Ctrl+C and SIGTERM stop the run gracefully: finished requests are reported,
  the rest are marked cancelled, `--output-file` is written, and the exit code
  is **130** (SIGTERM used to read as 143). A second signal exits at once.
- Command-line mistakes exit **64** instead of 1, with
  `Run 'hitspec <cmd> --help' for usage.` on stderr: an unknown flag or
  command, a bad flag value, a missing required flag, the wrong number of
  arguments, and `--dry-run` combined with `--stress` (which used to run the
  whole load test).
- These are now configuration errors (exit **3**, before any request is sent):
  - a missing or unreadable `--env-file` / `HITSPEC_ENV_FILE`. It used to be a
    warning, and the variables stayed unresolved;
  - a CA bundle, client certificate or key, or proxy URL that cannot be
    loaded. A bad proxy used to fail each request with exit 1;
  - a broken `hitspec.yaml`, in `validate`, `studio`, `serve` and `contract`
    as well as `run`. `studio` and `serve` used to fall back to the defaults
    without a word. The message is now `Error: <path>:<line>: ...`.
- `--dry-run` parses every file and prints the plan. A file that does not load
  now exits 2 (it used to exit 0 without parsing), and filters that match
  something exit 0 (they always exited 64). The plan is always text: `-o` and
  `--output-file` are ignored, so stdout is no longer an empty JSON or XML
  document.

**Which settings apply**

- `hitspec.yaml` is found by walking up from the working directory (from the
  workspace for `studio` and `serve`), stopping after the first directory that
  contains `.git`. A config in a parent directory of your tests now applies.
  `~/hitspec.yaml` and anything above your home directory are never read.
  `hitspec run -v` prints the file it used.
- `verbose: true` in `hitspec.yaml` now works for `run` (it is `-v`). The flag
  still wins: `--verbose=0` turns it off.
- Unknown keys in `hitspec.yaml` print a warning with a suggestion
  (`unknown key "tiemout" (did you mean "timeout"?)`).

**Requests sent differently**

- `? name = value` query lines keep the order you wrote them in, repeated
  names are all sent, and values are percent-encoded with `%20` for a space.
  They used to be sorted, collapsed to one value per name, and encoded with
  `+`. A server that cares about order or `+` sees a different URL. A name in
  both the URL and a `?` line is now sent twice (the line used to replace it).
- Characters that are not allowed in a URL (spaces, quotes, non-ASCII) are
  percent-encoded. They used to be sent raw, and most servers answered 400.
- `& name = value` form blocks keep the spaces in values (`John Smith` was
  sent as `JohnSmith`) and encode each name and value.
- A `< ./file` body or a multipart file that cannot be read fails the request
  with an error naming the path. The literal text `< ./file` used to be sent.
- A multipart file part without a name is sent with the field name `file`
  (it was empty). Name it with `file avatar @./me.png`.
- `@auth apiKeyQuery` replaces a parameter of the same name already in the
  URL instead of adding a second one.
- `@waitFor` polls with the run's own client, so it honors the proxy, CA,
  `--insecure`, default headers and `followRedirects: false`.
- Cookies are not kept between requests unless you ask (`--cookies`,
  `HITSPEC_COOKIES` or `cookies: true`).

**Reports**

- Failure output is longer: each failed request shows the request line, the
  response status and duration, and a body excerpt.
- `-v` prints `Response: 200 OK` where it printed `Status: 200`. `-vv` adds
  headers and `-vvv` adds full bodies.
- More header names are redacted (anything containing `token`, `secret`,
  `password` or `api-key`, such as `X-Csrf-Token`) in every format. The HTML
  report used to print headers verbatim.
- Capture misses and unresolved `{{variables}}` show as warnings under the
  request. They never change the exit code.
- JSON, JUnit and TAP gain fields (`warnings`, `response.bodyExcerpt`,
  `response.bodyTruncated`; longer failure text). JUnit states each failed
  assertion once (`status ==: expected 200, got 404`; it used to repeat the
  message). TAP escapes backslashes and newlines, so its YAML blocks parse. A
  check that matched the old text needs updating.

**Which files run**

- Directories are searched without `.git`, `node_modules` and `vendor`. Name a
  path inside one of them to run it.
- A file reached twice (named twice, or through a directory and a glob) runs
  once.
- An argument or glob that matches nothing reports
  `no .http or .hitspec files match "<arg>"`.

**mock, record, stress and metrics**

- `hitspec mock` and `hitspec record` listen on 127.0.0.1. For Docker or other
  machines, pass `--host 0.0.0.0`. A port that is already in use is an error.
- The Prometheus endpoint (`--metrics prometheus`) listens on 127.0.0.1. Pass
  `--metrics-host 0.0.0.0` (or `HITSPEC_METRICS_HOST`) when a Prometheus server
  on another machine or container scrapes it.
- `hitspec mock` answers a known path with the wrong method with 405 and an
  `Allow` header (it was 404), and serves HEAD for GET routes. CORS headers
  need `--cors`.
- `hitspec record` replaces passwords, tokens, secrets, API keys,
  `Set-Cookie` and `Proxy-Authorization` in recordings with placeholders
  (`--no-redact` keeps them), and keeps at most 1 MiB of each response body.
- Stress results count requests cut off at the end of the run as
  *incomplete*, not as errors or timeouts, so `rps` and `errors` thresholds
  can move. A non-2xx status that the request's own `expect status` accepts is
  no longer an error. Assertion failures have their own `assertions<N%`
  threshold and no longer count in `errors`.
- `errors<0`, `errors<=0` and a profile `errors: "0%"` mean "no request may
  fail". They used to switch the check off, so a run with errors passed.
- Profile thresholds keep their operator (`rps: ">=100"`, `p95: "<=250ms"`);
  a bare `rps` value means "at least". These used to be rejected.
- With `--profile`, flags you pass explicitly (`--rate`, `--duration`, ...)
  override the profile. They were ignored when they equalled the default.

**serve, studio and the desktop app**

- `--read-only` protects the workspace, not the network: running requests,
  stress, mock, record, contract verification, import and record export now
  work in read-only mode, and read-only sessions no longer save cookies.
  Writes to files, `hitspec.yaml`, environments, stress profiles and history
  are refused.
- A `< file` body or multipart file outside the workspace (including through
  a symlink) is refused in `serve`, `studio` and the desktop app, for runs,
  stress and contract verification. `hitspec run` is unaffected.
- Starting mock or record on a busy port fails (`409` from `serve`) instead of
  reporting "started" while nothing listens. A stress run that cannot start
  (a failed setup request, invalid settings) reports `error` instead of an
  empty, successful-looking result.
- `PUT /api/v1/config` applies a key that is present even when it is `0` or
  `""` (`retries: 0` now disables retries). For `timeout`, `retryDelay`,
  `maxRedirects` and `concurrency`, `0` resets to the default. Negative numbers
  return 400. The response is the whole resulting configuration. Clients that
  sent `0` or `""` to mean "no change" must leave the key out.
- On SIGINT or SIGTERM, `serve` waits up to 10 seconds for requests in
  flight; a second signal stops it at once.

**hitspec studio (terminal UI)**

These matter mostly to scripts and terminal-automation tests that drive
studio with keystrokes.

- `enter` no longer submits the forms on screens 2-9 (it used to start a
  30-second load test from screen 2). `enter` opens the fields and moves
  between them; submit with `s`, or `ctrl+s` while editing.
- The forms start empty, with the default shown as a placeholder. Some fields
  are now required (import input, record target URL, contract provider URL,
  cookie domain and name), and non-numeric numbers are rejected.
- `esc` on screens 2-9 goes back to the workspace.
- `ctrl+c` first cancels a run in flight, then asks before discarding unsaved
  edits; a second `ctrl+c` quits. `x` cancels a run.
- `r` and `R` run what is in the editor, including unsaved edits, not the file
  on disk. `ctrl+s` asks before overwriting a file that changed on disk, and
  opening another file (or importing, renaming, ...) asks when there are
  unsaved edits.
- Copy-as (curl, httpie, ...) resolves `{{variables}}` from the file, the
  active environment and the env file, so secrets such as tokens appear in
  the copied command. It works for requests without `@name`.
- The mouse is not captured unless you pass `--mouse`, so the terminal's own
  text selection works; mouse-wheel scrolling needs the flag.

### Security

- mock, record and the Prometheus metrics endpoint listen on loopback by
  default (see the upgrade notes).
- `serve`, studio and the desktop app refuse body and multipart files outside
  the workspace, including in stress and contract runs.
- The HTML report and every other format redact credentials in headers, and
  `hitspec record` redacts secrets in what it writes.
- The run history database and its sidecar files are created 0600 and
  `~/.hitspec` 0700; existing files are tightened when opened.
- hitspec Studio desktop: the bundled server exits within a second of the app
  dying (`serve --parent-pid`), and the stale-server cleanup on Windows checks
  the full command line before stopping a process.

### Added

- TLS and HTTP options on the command line, as `HITSPEC_*` variables and as
  `hitspec.yaml` keys, honored by run, studio, serve, stress and contract:
  `--cacert`, `--cert`/`--key` (mutual TLS), `--http1.1`, `--cookies`,
  `--max-response-bytes` and `--redirect-forward-headers`.
- `hitspec run -` reads a document from stdin; quoted `**` globs select files
  at any depth.
- `-v`, `-vv` and `-vvv` verbosity levels; `--dry-run` prints the plan
  (method, resolved URL, tags, dependencies, skip reasons).
- `--watch` re-runs on atomic saves (vim, JetBrains, ...), reloads
  `hitspec.yaml`, `--config` and the env file, and picks up new files.
- `@auth aws` takes the session token of temporary credentials as a fifth
  argument, and multipart bodies are signed correctly.
- `file <name> @path` names a multipart file part; the importers write it.
- `hitspec mock --host` and `--cors`; `hitspec record --host` and
  `--no-redact`; `--metrics-host`.
- Stress results report incomplete requests, assertion failures
  (`assertions<N%` threshold) and a warning when the target rate was not
  reached; Prometheus metrics update during the run.
- `serve --parent-pid`; `PUT /api/v1/config` accepts `retryDelay` and
  `maxRedirects`; `serve` and studio results and events carry `warnings`, and
  stress results carry the new counters and `error`.
- hitspec studio: `x` cancels a run, `<`/`>` step through a file run's
  responses, the header and assertion panels follow the selected request, and
  `--mouse` enables mouse capture.
- hitspec Studio desktop: launching the app again with a folder (or Open
  With) opens it in the running window, asking first about unsaved edits.

### Fixed

- `hitspec diff` writes through the command's output (it can be redirected and
  has no colors when not on a terminal), and `--output html` no longer fails.
- Exported curl and code snippets send `& key = value` form blocks with the
  same encoded bytes hitspec sends.
- `hitspec run` writes each run to the history in one transaction, and
  `serve`/studio commit a run before answering, so the run details are
  complete as soon as the run returns.
- `serve` no longer cuts `/run` and `/execute` off after 60 seconds, and
  shutting down drains requests instead of killing them.
- Snapshots are kept per run, so concurrent runs in `serve` no longer share
  them.
- Stress with `--metrics json` on a passing run wrote `total_requests: 0`.
- An edited `.env` file takes effect in long-lived processes (`run --watch`,
  the MCP server); variables set in your shell are never overwritten.
- hitspec studio: large JSON responses are pretty-printed instead of cut at
  24 KB, the request list fits an 80x24 terminal, and the Settings screen can
  set retries to 0.

### Changed

- The `-v` output format (see the upgrade notes). `snapshot.SetGlobalManager`
  is deprecated for library users; pass `assertions.WithSnapshotManager`.
- Dependencies: Bubble Tea 2.1.0, Lip Gloss 2.0.6, pflag 1.0.10, testify
  1.12.1 and x/time 0.16.0; the desktop app moves to Electron 44.9.0 and its
  lockfile drops a high-severity `http-cache-semantics` advisory.

## [2.21.0] - 2026-10-19

The `.http` language is now strict and assertions check what they say. Some
suites that were green on 2.20 will go red on 2.21 without any edit: in each
case the old result was wrong. **Read the upgrade notes below before you bump
hitspec in CI.**

### Upgrade notes: tests that may start failing

Every item below used to pass without checking anything, or ran with
settings other than the ones you configured. After upgrading, such a test
fails, or its file fails to load (exit code 2), even though the file did not
change. To find them before your CI does:

1. Run `hitspec validate <dir>` with the new version. It reports every file
   that no longer loads, with file:line:col and a fix, and prints warnings
   such as unknown directives. `hitspec validate --strict` turns the warnings
   into errors and also checks that every `{{variable}}` is defined.
2. Run the suite once locally with the new version and read the failures.
3. Pin the version in CI (the GitHub Action's `version:` input, or
   `go install .../cmd/hitspec@v2.21.0`) and bump it on purpose.

**Files that no longer load (exit code 2)**

- A misspelled operator was silently treated as `==`:
  `expect body.name contians "x"` now fails with
  `unknown operator "contians" (did you mean "contains"?)`.
- An operator without a value, or two values (`expect status 200 201`), is an
  error. Write `expect status in [200, 201]`.
- A `>>>`, `>>>capture`, `>>>db` or `>>>shell` block that is not under a
  request line (for example a capture block placed above the request), a block
  with an unknown type, or a block without its closing `<<<` was dropped
  without a word. Move the block below its request, fix the type, or close it.
- `>>>db <connection>` never existed: those blocks were ignored, so their
  checks never ran. Set the connection with `# @db <connection>` on the
  request and put the SQL and `expect <column> <op> <value>` lines inside
  `>>>db`.
- `jsonpath` features hitspec does not support (`..`, `?()` filters, slices,
  unions) are errors instead of silently matching nothing.

**Assertions that now check what they say**

- `== null` needs a JSON `null`; a missing field no longer equals null. Use
  `!exists` for "absent". Other value operators (`==`, `>`, `contains`, ...)
  fail on a missing field; the negated ones (`!=`, `!contains`, `!includes`,
  `!in`) pass.
- `body.x` on a response that is not JSON (HTML, plain text) fails with
  "response body is not JSON" instead of comparing against nothing.
  `application/problem+json`, other `+json` types and `text/json` count as
  JSON.
- Unquoted values are read whole: `expect body.name == Jane Doe` compares
  `Jane Doe` (it compared `Jane`), `./x` keeps its first character, and a
  trailing `# comment` is no longer part of the value.
- `jsonpath $.x` subjects are evaluated. They used to resolve to nothing, so
  `!exists` and `!=` passed whatever the response held.
- Objects and arrays compare by value (`expect body.user == {"id": 1}`) and
  numbers compare numerically.
- `length` counts characters, not bytes: `"café"` has length 4 (it was 5).
- `contains`, `startsWith`, `endsWith` and `matches` on a whole JSON body, an
  object or an array use its JSON text instead of Go's `map[...]` rendering.
- Paths with `.#` and `#(...)` work; the `#` used to start a comment and cut
  the path short.
- `# @if` / `# @unless` evaluate `==`, `!=`, `!`, `&&` and `||`. Before, any
  non-empty text was true, so `# @if {{env}} == prod` ran the request in every
  environment; it is now skipped outside prod.
- Directives above the first request of a file that has no `###` line were
  dropped. Its `@name`, `@tags`, `@skip` and `@depends` now apply, so tag
  filters and dependencies can select different requests.
- `>>>db` blocks written as bare SQL (without a `query` prefix) were dropped;
  they now run, and need `--allow-db`.

**Settings that now apply**

- `hitspec run` honors hitspec.yaml `timeout`, `retries`, `retryDelay`,
  `maxRedirects`, `parallel`, `concurrency` and `bail`; the flag defaults
  (30s, concurrency 5) used to override them. A yaml `timeout: 5000` now fails
  requests slower than 5 seconds, `retries: 3` repeats failing requests and
  `bail: true` stops at the first failure.
- `HITSPEC_INSECURE=false`, `HITSPEC_PARALLEL=false` and `HITSPEC_BAIL=false`
  override hitspec.yaml. The order everywhere is: flag, `HITSPEC_*` variable,
  hitspec.yaml, built-in default.
- `maxRedirects: N` follows N redirects; it followed N-1.

**HTTP behavior**

- `HTTP_PROXY`, `HTTPS_PROXY` and `NO_PROXY` (and the lowercase forms) are
  honored when no proxy is configured. On a machine or runner with a proxy in
  its environment, requests now go through it; list hosts that must be reached
  directly in `NO_PROXY`. Loopback hosts are never proxied.
- On a redirect to another scheme, host or port, headers you set
  (`Authorization`, `Cookie`, `X-Api-Key`, ...) are no longer forwarded, so
  they cannot leak to another origin. A test that relied on the credential
  following the redirect gets a 401: point it at the final URL.
- A `Host:` header is sent as the request's Host; it used to be ignored.
- With an explicit `Accept-Encoding: gzip` or `deflate`, the body is decoded
  before assertions; they used to see compressed bytes.
- Several `Set-Cookie` headers are joined with `, ` (it was `; `) in
  `header Set-Cookie` assertions, like every other repeated header.
- Response bodies over 100 MiB fail with "response body exceeds".
- HTTP/2 is negotiated over TLS.

**Other commands**

- `hitspec diff`: skipped tests have their own status and no longer count as
  failures or regressions; a 0 ms baseline never trips `--threshold`.
- `hitspec mcp serve`: saved requests run by `hitspec_fetch` no longer read the
  server's environment variables; allow one with `--allow-env NAME`.
  `--workspace` refuses your home directory, its parents and `/` unless you
  pass `--allow-broad-workspace`. Run the MCP canary before switching a
  gateway to this version.

### Security

- `hitspec mcp serve`: saved requests resolve variables in an isolated
  environment (process secrets such as `TAVILY_API_KEY` stay out of reach),
  `hitspec_fetch` is annotated as destructive, and broad workspaces are
  refused (see the upgrade notes).
- Credentials no longer follow cross-origin redirects (see the upgrade notes).
- OAuth2 token requests honor the request timeout (30s cap) and read at most
  1 MB; they used to have no timeout at all.
- hitspec Studio (desktop): the renderer can no longer set the server's
  environment or `--allow-shell` (shell access needs approval in a native
  dialog, per workspace, until quit); every IPC call is checked against an
  allowlist of senders, methods and paths; Electron fuses disable
  `ELECTRON_RUN_AS_NODE`, `NODE_OPTIONS` and `--inspect` and enforce ASAR
  integrity; the CSP no longer allows inline styles.
- CI blocks on high-severity gosec findings.

### Added

- `hitspec validate --strict`: warnings become errors and `{{variables}}` are
  checked against the file, its captures, hitspec.yaml environments, `.env`
  files and the process environment. Parse warnings (unknown directives with
  a did-you-mean, bad `@timeout` values, directives that apply to nothing,
  capture paths with spaces) carry file:line:col.
- `hitspec export` writes fetch, wget, python, httpie, go and ruby as well as
  curl (`hitspec export --format python api.http` or
  `hitspec export python api.http`), including query parameters, `@auth` and
  GraphQL bodies.
- `hitspec import openapi -o dir/` writes one file per tag. Postman and
  Insomnia environments become hitspec.yaml environments; Postman collection
  variables, inherited auth and common `pm.test` checks are converted.
- `jsonpath` subjects and captures (`$`, `.a`, `['a']`, `[n]`, `[*]`); every
  documented form of `each`; `/re/flags` literals for `matches`.
- `@if` / `@unless` comparisons with `==`, `!=`, `!`, `&&`, `||`.
- Runs started from studio or `serve --api-only` stop when cancelled or when
  the client disconnects; requests that never started are reported as
  skipped ("cancelled").
- hitspec Studio desktop: unsaved-edit guards (switching files, closing,
  quitting, external changes), a per-architecture bundled CLI, and a desktop
  CI workflow. Installers are unsigned until signing credentials are set up.
- Every example under `examples/` runs offline against `hitspec mock`.

### Fixed

- Studio and `serve --api-only` ignored most of hitspec.yaml (timeout, proxy,
  headers, SSL, redirects, retries) and the `HITSPEC_*` variables; stress and
  contract runs used a bare HTTP client. Every surface now builds its settings
  the same way.
- `>>>db` SQL, `>>>capture` paths and `>>>shell` commands keep their quotes,
  spacing and `{{braces}}`.
- OAuth2 tokens are cached per client until shortly before they expire,
  fetched once under concurrency, and refetched once when a cached token gets
  a 401. A token response without `access_token` is an error.
- AWS SigV4 signs `x-amz-content-sha256`, the session token
  (`AWS_SESSION_TOKEN`) and `x-amz-*` headers, encodes paths and query strings
  per RFC 3986, and no longer panics on a request without headers.
- Importers stopped dropping input: curl (`--json`, `-F`, `-d @file`,
  repeated `-d`, `$'...'` quoting, header order), Postman (form-data, GraphQL
  and file bodies, `:path` variables), Insomnia (form, multipart, GraphQL and
  file bodies, apikey and digest auth) and OpenAPI (path parameter values,
  multipart bodies, stable output, no panic on untyped parameters). Large
  numbers stay plain (`1234567`).
- `hitspec export curl` sends form bodies with `-d` (they were URL-encoded a
  second time) and file bodies with `--data-binary @file`.
- `hitspec diff` lists tests in file and request order (it was random) and
  keeps duplicate names apart.
- `hitspec record` no longer writes assertions that fail to parse for JSON
  keys such as `first name`.
- hitspec Studio desktop: a CLI that cannot start (permissions, quarantine,
  wrong CPU) reports why instead of hanging; a server left behind by a crash
  is cleaned up on the next launch; the server version is shown and old CLIs
  are flagged; the log view no longer freezes; bodies over 512 KB render as
  plain text with a Highlight button.
- Editors: the Neovim plugin installs with lazy.nvim and packer; the Neovim
  and VS Code grammars no longer highlight syntax hitspec does not have.
- Docs: samples that the parser rejects or that never worked (database
  guides, troubleshooting, captures) are fixed, and CI parses every `http`
  block in the docs.

### Changed

- HTTP/2, proxy variables and the 100 MiB body limit are on by default (see
  the upgrade notes).
- Releases run the full test suite and a smoke test of the built binary
  before publishing, and the GitHub release notes are this changelog.

## [2.20.0] - 2026-10-19

This release fixes the regressions and false passes found in the 2026-10-19
review. Upgrade if you use `hitspec run` in CI: v2.16.0 through v2.19.1 print
nothing on stdout, and until now a file that failed to parse did not fail the
run.

### Upgrade notes: tests that may start failing

These fixes can turn a green run red without any change to your files; in
each case the old result was wrong. Run the suite once locally with 2.20
before upgrading CI.

- `expect header X exists` passed even when the header was missing; it now
  fails in that case.
- A response body cut short by a timeout (`@timeout`, `--timeout`) counted as
  a success with a partial body; it is now a failure. Only
  `text/event-stream` (SSE) responses may end at the deadline.
- A file that fails to load (syntax error, duplicate `@name`, circular
  `@depends`) was skipped and the run could still exit 0; `hitspec run` now
  exits 2. `--name`/`--tags` filters that match nothing exit 64.
- `@auth` with an unknown type or the wrong number of arguments is a parse
  error.
- Without `--env` or `HITSPEC_ENV`, `hitspec run` uses hitspec.yaml
  `defaultEnvironment` (it always used `dev`), so variables such as
  `{{baseUrl}}` can resolve differently.
- Header values are sent verbatim: quotes and text after `#` are kept
  (`If-None-Match: "abc"` sends the quotes).
- Captured values are inserted literally; a `{{...}}` inside a captured value
  is no longer expanded.

### Security

- `serve --api-only` could be driven by any web page. A cross-origin
  `text/plain` POST (no CORS preflight) reached `POST /contract/verify`,
  whose `stateHandler` ran an arbitrary executable. The server now:
  - rejects `stateHandler` unless started with `--allow-shell`, and then only
    for a script inside the workspace (403 otherwise);
  - requires `Content-Type: application/json` on JSON bodies (415);
  - accepts only `localhost`, `127.0.0.1` and `[::1]` as Host on loopback binds
    (DNS rebinding);
  - rejects requests and WebSocket upgrades from foreign Origins;
  - generates and prints an API token when bound beyond loopback or started
    with `--allow-shell`/`--allow-db`;
  - compares tokens in constant time and accepts `bearer` in any case;
  - answers CORS preflights before authentication.
- `HITSPEC_API_TOKEN` is read at runtime instead of being the `--api-token`
  default, which printed the secret in `--help`. The desktop app no longer
  passes its token on the command line, where `ps` showed it.
- Captured response values are inserted literally. A response containing
  `{{$env(SECRET)}}` or `{{HOME}}` used to be expanded and sent in the next
  request.
- The GitHub Action no longer uses `eval` or interpolates inputs into its
  script, and verifies the downloaded archive against the release checksums.
- Build with Go 1.26.9 and `golang.org/x/net` v0.61.0 to pick up the fix for
  GO-2026-6617 (HTTP/2 HPACK encoder race in `net/http`).

### Fixed

- `hitspec run` printed nothing on stdout for every output format unless
  `--output-file` was set, and machine formats failed with "invalid argument".
  Config errors now print a message naming the file instead of exiting 3
  silently.
- `hitspec run` exits 2 when a file cannot be loaded (syntax error, duplicate
  `@name`, `@depends` cycle) instead of 0; the other files still run and the
  error is reported on stderr and in json/junit/tap/html.
- `--name`/`--tags` that match no request exit 64 with a message instead of
  passing, and a selected request's `@depends` targets run even when the
  filter excluded them.
- `--quiet` still writes `--output-file` and lists failures on stderr.
- Watch mode recreates `--output-file` on every rerun (one valid document)
  and prints its banners on stderr.
- `run` honors `defaultEnvironment` from hitspec.yaml (order: `--env`,
  `HITSPEC_ENV`, `defaultEnvironment`, `dev`).
- `@baseUrl = {{baseUrl}}` resolves against the environment (hitspec.yaml, then
  .env, then the OS) instead of being treated as a cycle.
- Captured numbers keep their exact JSON form (`1000000`, not `1e+06`);
  objects and arrays are inserted as compact JSON and null as `null`.
- `hitspec init` scaffolds a project that passes against `hitspec mock`, and
  its "Next steps" commands work. The mock server accepts a leading
  `{{baseUrl}}` and dotted placeholders in routes.
- Parser regressions from v2.16.0:
  - a comment line in the header block turned the remaining headers into body;
  - the request line was split on spaces inside `{{ }}`
    (`{{$urlEncode(hello world)}}`, `{{ base }}`);
  - header values lost quotes and everything after `#`
    (`If-None-Match: "abc"`, `X-Color: #fff`);
  - an unclosed `'` swallowed the rest of the file.
- `@auth` arguments separated by commas (the documented form) work for every
  auth type; spaces work too. Unknown auth types and wrong argument counts
  are parse errors.
- `expect header X exists` passed when the header was missing.
- A response body cut short by a timeout counted as success; the deadline is
  now tolerated only for `text/event-stream` responses.
- `hitspec studio --env` and `--config` were ignored. Stress runs, mock and
  contract verification in studio resolve `{{variables}}` from the active
  environment, and engine output (stress progress, mock and proxy logs) no
  longer paints over the UI.
- `hitspec record` now forwards to the target URL's path prefix
  (`--target http://host/api` sends `/users` upstream as `/api/users`), matching
  the URL it records, so replaying a recording hits the same endpoint. The
  proxy uses `ReverseProxy.Rewrite` (the `Director` hook is deprecated).
- The studio and `serve` file watchers now watch every directory created after
  startup, including nested ones (`mkdir -p`), and report `.http` files that
  already exist inside a directory moved or copied into the workspace
  (`cp -R`, `git checkout`, unzip). Debounce timers are guarded by a mutex and
  removed once they fire. `.git`, `node_modules`, and `vendor` are no longer
  watched.

### Changed

- The Go module path is `github.com/abdul-hamid-achik/hitspec/v2`. Install with
  `go install github.com/abdul-hamid-achik/hitspec/v2/cmd/hitspec@latest`
  (the old path resolved to v1.3.0). A clean checkout builds without
  `go generate`, and `hitspec version` reports the module version when built
  with `go install`.
- GitHub Action: use `abdul-hamid-achik/hitspec@v2`, a tag that now follows
  every stable release. `files` accepts several paths and globs, `timeout`
  accepts milliseconds or durations, `output` defaults to `console` (machine
  formats write `hitspec-results.<ext>`, exposed as the `report-file` output),
  `env` is passed only when set, and Windows and arm64 runners are supported.
  `@v1` is unchanged.
- `serve --api-only` clients must send `Content-Type: application/json` with
  JSON bodies.

## [2.19.1] - 2026-07-19

### Fixed

- MCP initialization instructions now match the tools and effective
  `hitspec_fetch` network authority configured for the server, instead of
  recommending unavailable capabilities or always describing direct URLs as
  public-only.

### Security

- When the operator enables `--allow-private-network`, initialization help and
  public documentation now describe the full non-public destination authority,
  including private, loopback, link-local, and reserved addresses. The
  public-only default and webpage-capture policy are unchanged.

## [2.19.0] - 2026-07-18

### Added

- Added `hitspec mcp probe` for CI-friendly MCP lifecycle and paginated tool
  discovery checks over stdio and Streamable HTTP. Required tools can be
  asserted with repeatable `--require-tool` flags.
- Added `hitspec mcp call` for explicit tool invocations with inline or
  file-backed JSON arguments, advertised input/output JSON Schema validation,
  stable JSON reports, and distinct failures for protocol errors, contract
  mismatches, and MCP `isError` results.

### Documentation

- Expanded the MCP reference and LLM documentation with client examples,
  transport flags, exit behavior, and security boundaries.

### Security

- MCP stdio targets execute direct argv without a shell. HTTP client probes
  reject redirects, URL credentials, and transport-managed header overrides;
  human output strips terminal control characters.

## [2.18.1] - 2026-07-16

### Fixed

- Cleared the macOS Gatekeeper quarantine attribute on the Homebrew cask
  install so `brew install --cask abdul-hamid-achik/tap/hitspec` binaries run
  without a manual `xattr` step.

## [2.18.0] - 2026-07-13

### Added

- Added the provider-neutral `hitspec_search_web` tool with a bounded Tavily
  adapter. It returns at most 64 KiB of normalized discovery candidates and
  keeps provider credentials and upstream request details server-side.
- Added `hitspec_capture_webpage` with the compatibility-preserving
  `url`/`name`/`tags`/`ttl`/`index` input and compact file.cheap receipt. The
  tool preserves webpage Markdown and its content hash without returning the
  full page to the model.

### Changed

- Defined the stable Local Agent web surface as `hitspec_search_web`,
  `hitspec_fetch`, and `hitspec_capture_webpage`. Tavily is an interchangeable
  internal provider, and `hitspec_fetch` remains retrieval-only.
- Updated CI, release, and example workflows to Node.js 24-based action majors:
  checkout v6, setup-go/setup-node v6, setup-task v3, golangci-lint v9,
  Codecov v6, upload-artifact v7, and GoReleaser Action v7. This removes the
  deprecated Node.js runtime alert from current GitHub Actions runs.

### Documentation

- Documented the Local Agent web surface (search, fetch, and webpage capture),
  provider isolation, tvault secret handling, approval boundaries, capture
  compatibility, and the canary rollout.
- Updated copyable GitHub Actions examples to the same Node.js 24-compatible
  action majors and replaced the Node-based legacy test reporter example with
  the Docker-based publisher already used elsewhere in the docs.

### Security

- Search provider endpoints, credentials, the file.cheap executable, and stash
  paths are server-owned. The file.cheap adapter uses a private mode-0700
  handoff, direct argv execution, sanitized provenance, bounded child output,
  and an allowlisted child environment that excludes provider credentials.
- Removed the final direct GitHub Action input interpolation from the install
  script; the requested Hitspec version now crosses the shell boundary through
  a quoted environment variable.

## [2.17.0] - 2026-07-13

### Added

- **Single-response fetching**: added `hitspec fetch <url|file>` for executing one ad-hoc URL or one saved request and emitting `raw`, readable `text`, self-contained `markdown`, or a machine-safe `json` envelope. The command supports bounded response sizes, timeouts, redirects, TLS and proxy options, environment selection, HTTP failure semantics, and atomic mode-`0600` file output.
- **Model Context Protocol server**: added `hitspec mcp serve` with `hitspec_fetch`, `hitspec_list_requests`, and `hitspec_validate` tools over stdio. The server is compatible with MCPHub namespacing and Local Agent text-content handling.

### Changed

- HTTP responses can now enforce a maximum body size and expose their final URL. Runner execution accepts a context so cancellation reaches in-flight requests and retry waits.

### Security

- MCP file access is confined to a fixed workspace after symlink resolution. Network requests allow only public destinations by default, including DNS, redirects, and dial targets; private or loopback access requires the server operator to pass `--allow-private-network`.
- URLs with embedded credentials are rejected, response provenance omits query strings, sensitive response headers are omitted from JSON output, and raw MCP payloads use base64 envelopes.

### Documentation

- Added CLI and MCP references plus an agent-artifact workflow for file.cheap, MCPHub, and Local Agent. TinyVault integration is explicitly deferred pending a separate secret-reference and redaction contract.

## [2.16.0] - 2026-07-10

### Fixed

- Hardened parser, environment, runner, HTTP, stress, serve, mock, proxy, CLI,
  and terminal-client behavior after a deep correctness audit. Critical fixes
  include `init`/`run` nil-dereference prevention, correct header parsing, and
  deterministic request ordering.

### Security

- Prevented GitHub Action input injection, required API authentication where
  configured, created SQLite files with private permissions, and hardened the
  GoReleaser workflow.

## [2.15.0] - 2026-06-16

### Added

- **`>>>mock` response blocks**: a request can define an exact mock-server response body in a `>>>mock ... <<<` block, returned verbatim with a `Content-Type` inferred from the body (JSON / XML / HTML / text). Without a `>>>mock` block, the mock server still infers the response from assertions. Added `>>>mock` syntax highlighting in the VSCode and Neovim grammars, plus an `examples/mock/` example.
- **Full code export**: copying a request as HTTPie, Python, fetch, Go, Ruby, or wget (command palette) now emits a complete snippet including the method, URL, headers, and body — previously only the method and URL.

### Fixed

- **Request body no longer swallows a trailing `>>>db` / `>>>shell` block**: a `>>>db` or `>>>shell` block placed directly after a request body (with no intervening assertion block) was consumed into the body, silently dropping its assertions. The body parser now stops at those block delimiters.
- **Unclosed `>>>mock` / `>>>graphql` blocks now report a parse error** with line info (`unclosed >>>mock block (missing closing <<<)`) instead of silently consuming the rest of the file.
- **Raw block content may contain `>>>` / `<<<` mid-line** (e.g. inside a JSON string in a `>>>mock` body); only a delimiter at the start of a line ends the block.

### Documentation

- Added a dedicated **Studio (terminal UI)** page, a **Build from Source** section in installation, and `completion` / `version` CLI reference pages; wired the orphaned REST API reference into the navigation.
- Corrected inaccuracies verified against the code and by running the examples: removed the never-loaded `.hitspec.env.json` environment model (environments come from `hitspec.yaml`), fixed `@auth` directive placement, made the quickstart example actually pass, corrected the `--timeout` / `--insecure` flags, snapshot file paths, binary-download URLs, the GitHub Action input/env tables, and the assertion-operator count (26). Regenerated `llms.txt` / `hitspec docs` and removed a stale Vue/SPA section.
- The Mintlify docs now pass `mintlify broken-links` cleanly (fixed an MDX syntax error and a broken navigation link); added a `task docs:check` target to catch these in CI.

## [2.14.1] - 2026-06-15

### Fixed

- **`studio` TUI layout and input**: bounded the workspace layout so panes no longer overflow on small terminals, fixed form-field input handling, and made `ctrl+c` quit reliably.

### Changed

- Hardened the end-to-end suite: reorganized glyphrun specs into `flows`/`actions`/`fixtures`, isolated `HOME` from real user data, fixed file-watcher status-clobber flakes, and added stress/record/import/clear-history flows. Dropped the redundant cairntrace browser suite. Raised TUI unit coverage to ~85%.

## [2.14.0] - 2026-06-14

### Changed

- **API Client Manager is now a native interactive terminal app** launched with **`hitspec studio`** (the Vue.js web SPA was removed and replaced by a keyboard-first Charm Bubble Tea v2 app). `hitspec serve --api-only` runs the REST/WebSocket API server; `hitspec serve` without `--api-only` still opens the app for backward compatibility but prints a hint to use `hitspec studio`. The app is driven by a new in-process `clientmgr` facade shared by both surfaces.
- **Removed the deprecated `--open` flag** from `serve`, and scrubbed user-facing "TUI"/"web-based" jargon from CLI help, `init` next-steps, README, and docs.

### Added

- **Tabbed response viewer**: Body / Headers / Assertions / Timing / Captures, with pretty-printed and syntax-highlighted JSON (chroma) and status colored by class.
- **Copy/export as code**: render the selected request as curl / HTTPie / Python / fetch / Go to the system clipboard, plus copy the response body (command palette).
- **Toast notifications**: severity-colored, auto-dismissing — errors and successes no longer vanish on the next keypress.
- **Confirm dialogs**: destructive actions (delete file/cookie/run, clear recordings/history) now require confirmation.
- **Environment switcher** (`ctrl+e`) and **environment-variable editing** on the settings screen (read-modify-write so other variables are preserved).
- **Interactive run history**: select a run, drill into per-request details (`GetRun`), and delete runs.
- **Full stress results**: percentiles and per-request breakdown shown after a load test completes.
- **Color themes**: choose between Nord, Catppuccin Mocha, Dracula, Tokyo Night, and Gruvbox Dark via the theme picker (`ctrl+t`), the command palette, or the `--theme` flag. Per-method color badges and status-by-class coloring track the active theme.
- **Welcoming first-run experience**: an empty workspace now shows a centered welcome card with next steps, including a one-key "generate sample project" (`g`) that scaffolds `hitspec.yaml` + `example.http` (shared with `hitspec init`).
- **Centered keyboard help overlay** (`?`): a sectioned, aligned shortcut reference replaces the cramped single-line help.
- **Screen navigation strip + context-aware hints**: a numbered (1–9) screen tab strip and a status bar that shows the keys relevant to the focused pane.

### Fixed

- **studio input routing**: a key that opened a modal or moved focus into an editor is no longer re-delivered to that widget; modals now capture all input.
- **Pure render path**: the stress/mock/record screens no longer perform manager I/O during `View()` (status is cached on the update path).
- **Errors persist until resolved**: a status-bar error no longer vanishes on the next keypress — it stays until you navigate or the next operation succeeds, and is truncated to fit instead of overflowing.
- **Quit protects unsaved work**: pressing `q` with unsaved edits now asks for confirmation (`ctrl+c` still hard-quits); the command palette and overlays render consistently over the workspace; mock/record screens and the search overlay show actionable guidance and result counts; secondary form fields and viewports adapt to small terminals.
- **Cookie capture**: `Set-Cookie` headers from responses are now stored correctly (capture previously read redacted headers and never recorded a cookie).
- **Stale selection**: deleting the selected file no longer leaves its name in the top bar/source pane.
- **History database**: enabled `busy_timeout` and a single connection to avoid `SQLITE_BUSY` when run recording overlaps reads/deletes.
- **Stress metrics**: fixed a data race between `StressStatus` and a running test.

## [2.13.0] - 2026-02-27

### Fixed

- **Neovim: Operators not highlighted**: `syn keyword` silently ignored non-keyword chars (`==`, `!=`); switched to `syn match`
- **Neovim: Assertion subjects never highlighted**: `containedin=` on a `syn keyword` is ineffective; redesigned as full-line `syn match` with `contains=`
- **Neovim: `contains` keyword error**: `contains` is a reserved Vim syntax argument; switched operator keywords to `syn match`
- **VSCode: Separators rendered as comments**: Comment pattern matched `###` lines before separator pattern could; fixed pattern ordering
- **VSCode: Symbolic operators never matched**: `\b` word boundary fails on `==`, `!=`, `>=`, `<=`, `>`, `<`; split into separate patterns
- **Both: `@waitFor` not recognized**: Annotation pattern used lowercase `waitfor`; fixed to match camelCase `waitFor`
- **Both: Query params/form data not highlighted**: Pattern required leading whitespace; real files use column 0
- **VSCode: Global operator false positives**: Removed top-level operators rule that matched `=`, `>`, `<` everywhere

### Added

- **Neovim/VSCode: File include syntax**: Highlighting for `< ./path/to/file` body references
- **Neovim/VSCode: Type keywords**: `string`, `number`, `boolean`, `array`, `object` in assertions
- **Neovim: Multipart keywords**: `field` and `file` keywords in `>>>multipart` blocks
- **Neovim: Database keyword**: `query` keyword in `>>>db` blocks
- **Both: Negative number support**: `-42` and `-3.14` now highlighted as numbers
- **VSCode: Boolean and null literals**: `true`, `false`, `null` highlighting
- **VSCode: Legacy capture markers**: `[[[` and `]]]` block delimiters
- **VSCode: `headers` (plural)**: Added as assertion subject alongside `header`
- **Neovim: Variables in annotation values**: `{{var}}` inside `# @auth bearer {{token}}` now highlighted
- **Neovim: Assertion operators scoped**: Operators like `contains`, `type`, `in` now only match inside `expect` lines, preventing false positives in JSON bodies

### Changed

- VSCode built-in functions list trimmed to actual parser builtins (was bloated with 100+ nonexistent functions)

## [2.12.0] - 2026-02-27

### Fixed

- Format `types.go` with gofmt
- Update FileTree component tests for `treeitem` role

## [2.11.0] - 2026-02-11

### Added

- **File Editor Write-Back**: New Source tab in the request panel with a CodeMirror editor for editing `.http` files directly in the UI
  - `Cmd+S` / `Ctrl+S` save shortcut with dirty state indicators
  - PUT, POST, DELETE file endpoints with workspace path validation
  - File watcher self-write suppression to prevent reload loops
- **Stress Test Results Panel**: Full results dashboard shown after a stress test completes
  - Summary cards: total requests, avg RPS, success rate, errors
  - Latency percentiles: min, P50, P95, P99, max, mean, stddev
  - Per-request breakdown table with individual latency metrics
  - Threshold pass/fail indicators
  - "Run Again" button to return to config view
  - `GET /api/v1/stress/result` endpoint for fetching last test result
- **Settings Persistence**: Settings changes in the UI now persist to `hitspec.yaml` on disk
  - `PUT /api/v1/config` writes changes to the config file
  - Creates `hitspec.yaml` if none exists
- **Stress Profile CRUD**: Create, edit, and delete stress profiles from the UI
  - `POST /api/v1/stress/profiles` to create a profile
  - `PUT /api/v1/stress/profiles/{name}` to update a profile
  - `DELETE /api/v1/stress/profiles/{name}` to delete a profile
  - All changes persist to `hitspec.yaml`
  - Profile selection populates stress config fields in the UI

### Fixed

- **Diff Coloring**: Increase body preview limit from 512 bytes to 64KB so response diffs render correctly
- **Stress Test Latency**: Use fractional milliseconds (`float64(d.Microseconds()) / 1000.0`) instead of truncating integer milliseconds
- **Stop Button State**: WebSocket `stress_update` events now include a `running` boolean so the UI updates when a test finishes naturally
- **Stop Endpoint**: Return `200 OK` with `"already_stopped"` status instead of `400 Bad Request` when no test is running

## [2.10.1] - 2026-02-10

### Fixed

- Scope execution progress WebSocket events to current `execId` to prevent stale updates
- Add auto-scroll to execution progress list

## [2.10.0] - 2026-02-10

### Added

- **CI Pipeline Overhaul**: Parallel fan-out for lint, test, and security scan stages
- **Unified Taskfile**: Cross-stack `task build/test/lint/security/check` covering Go and TypeScript
- **Client Linting**: oxlint with Vue plugin (0 errors, 0 warnings on 93 files)
- **Editor Extensions**: VSCode extension at `apps/vscode/` with syntax highlighting and snippets, Neovim plugin at `apps/nvim/`
- **Neovim Support**: Vim syntax highlighting and filetype detection for `.http` and `.hitspec` files

### Changed

- Migrate Mintlify docs from `mint.json` to `docs.json` (v2 format)
- Pin Go version in `.tool-versions` for reproducible local builds
- Format all 29 Go files with `gofmt` for consistency

## [2.9.0] - 2026-02-10

### Fixed

- **GraphQL Variable Parsing**: Fix GraphQL variable parsing in the parser
- **Frontend UI Improvements**: Various UI polish in the web client

### Changed

- Build frontend in CI, remove dist from git tracking

## [2.8.0] - 2026-02-10

### Added

- **Live Execution Progress**: Per-request execution progress via WebSocket, showing real-time status during test runs

## [2.7.0] - 2026-02-10

### Fixed

- **WebSocket and SSE Support**: Delegate `Hijack`/`Flush` in `statusWriter` for proper WebSocket and SSE proxying
- Remove unused `requestIDFromContext` to pass lint

### Added

- **Single-Request Execution**: Fix and wire single-request execution from the UI
- **Export Wiring**: Wire export commands end-to-end
- **Structured Logging**: Add structured logging support

## [2.6.0] - 2026-02-10

### Added

- **Persistent Run History**: Record test runs to SQLite via `sqlc` + `modernc.org/sqlite` (pure Go, no CGo)
  - Schema: `runs` -> `results` -> `assertions` with CASCADE deletes
  - CLI flags: `--history-db`, `--no-history`
  - Server endpoints: `GET/DELETE /api/runs`, `GET /api/runs/:id`
  - Client: paginated history view with run/result/assertion drill-down
  - Non-blocking goroutine recording (doesn't slow test execution)

### Fixed

- File tree collapse not working due to missing reactivity trigger
- Rebuilt embedded client assets with history feature

## [2.5.0] - 2026-02-09

### Added

- **CLI DX Improvements**: Better CLI developer experience with improved defaults and help text
- **Parser Split**: Split parser into 6 focused files for maintainability
- **Runner Split**: Split runner into 4 files (runner, execute, filter, condition)

### Fixed

- Documentation accuracy and auth bug fixes
- UX polish across CLI and web client

## [2.4.1] - 2026-02-09

### Fixed

- **UTF-8 Lexer**: Lexer now handles multi-byte UTF-8 characters correctly (rune-based)
- **Dead Code Cleanup**: Remove unused code across packages
- **Contract Alignment**: Align contract testing with spec

### Added

- 112 new tests across packages

## [2.4.0] - 2026-02-09

### Added

- **Run Results Explorer**: Browse and inspect past test run results in the web client
- **Export Dialog**: UI dialog for exporting requests to curl, wget, and httpie
- **Sidebar Request Navigation**: Navigate requests directly from the sidebar

## [2.3.0] - 2026-02-09

### Fixed

- Add `doc.go` package comments to all internal packages for linter compliance

### Added

- **P0 Feature Wiring**: Wire all priority-0 features end-to-end
- **Security Hardening**: Input validation, path traversal protection, secure defaults
- **UX Polish**: Improved error messages, loading states, and feedback
- **Docs Accuracy**: Fix documentation to match actual behavior

## [2.2.1] - 2026-02-09

### Fixed

- **Race Conditions**: Fix data races in concurrent test execution
- **Dead Code**: Remove unused functions and types
- **Missing Endpoints**: Add endpoints that were documented but not implemented
- **E2E Fixes**: Fix end-to-end test reliability
- Resolve 63 bugs across client and server from two-pass audit

## [2.2.0] - 2026-02-09

### Added

- **Major Client Enhancement**: Redesigned web client with improved editor DX
- **Documentation Expansion**: Comprehensive docs for all features
- **Security Fixes**: Address OWASP top-10 issues in server

## [2.1.0] - 2026-02-09

### Added

- **Run Results Explorer**: Browse and inspect test run results
- **Export Dialog**: Export requests to curl, wget, httpie from the UI
- **Sidebar Request Navigation**: Quick navigation to requests

## [2.0.2] - 2026-02-09

### Fixed

- Use `defaultEnvironment` from `hitspec.yaml` when `--env` is not set
- Align execute API fields between client and server
- Add "Run All" button and clear history functionality

## [2.0.1] - 2026-02-09

### Fixed

- Remove unused `rootFile` type to satisfy linter
- Return file tree structure from workspace API so UI renders files correctly

## [2.0.0] - 2026-02-09

### Added

- **Web Client (hitspec serve)**: Browser-based API Client Manager
  - Built with Vue 3, Pinia, Reka UI, and CodeMirror
  - Embedded into Go binary via `go:embed`
  - Proxy to Vite dev server in dev mode
- **Mintlify Documentation Site**: Full documentation at `apps/docs/`
- **hitspec serve Command**: Start a local server with the web client

### Fixed

- Pin Go 1.25.7 to resolve crypto/tls vulnerability GO-2026-4337
- Resolve lint issues in serve package
- Fix `docs.json` for new Mintlify v2 schema

## [1.3.0] - 2026-02-05

### Added

- **Security Flags**: `--insecure` and TLS configuration options
- **DX Improvements**: Better error messages, auto-detection of file format
- **Mintlify Documentation**: Initial documentation site setup

## [1.2.3] - 2026-02-04

### Added

- **Export to Curl**: New `hitspec export curl` command to convert `.http` files to executable curl commands
  - Filter by request name with glob patterns (`--name "Login*"`)
  - Filter by tags (`--tags smoke,auth`)
  - Output to file (`-o commands.sh`) or stdout
  - Execute curl directly (`--exec`, requires single request)
  - Verbose curl output (`--verbose`)
  - Environment variable resolution (`--env staging`)
  - Handles all auth types (bearer, basic, apiKey, digest)
  - Handles all body types (JSON, form, multipart, GraphQL)

### Fixed

- **Escaped Quotes in JSON Body**: Escaped characters (like `\"`) inside JSON body values are now correctly preserved
  - Previously, `{"content": "{\"test\": true}"}` would become malformed JSON
  - Now escape sequences (`\"`, `\\`, `\n`, `\t`, etc.) are properly maintained when sent to the server
  - This fixes issues with APIs that accept JSON-in-JSON payloads

## [1.2.2] - 2026-02-04

### Changed

- Updated README with length comparison operators documentation
- Updated CHANGELOG with v1.2.1 release notes

## [1.2.1] - 2026-02-04

### Added

- **Length Comparison Operators**: Support comparison operators with `length` assertions
  - `expect body.items length > 0` - greater than
  - `expect body.items length >= 1` - greater than or equal
  - `expect body.items length < 100` - less than
  - `expect body.items length <= 50` - less than or equal

### Fixed

- **Variable Interpolation in Expect Clauses**: Captured variables like `{{createItem.itemId}}` now resolve in expect clause expected values
  - Previously, variables in expected values were compared as literal strings
  - Now `expect body.id == "{{login.userId}}"` correctly compares against the captured value

## [1.2.0] - 2026-01-20

### Added

- **Snapshot Testing**: Assert response bodies against saved snapshots with `expect body snapshot "name"` syntax
  - Store snapshots in `__snapshots__/` directory
  - Update with `--update-snapshots` flag
- **curl Import**: Import curl commands to hitspec format with `hitspec import curl "curl ..."`
  - Supports common flags: `-X`, `-H`, `-d`, `--data`, `-u`, `-k`, `-L`
  - Convert clipboard or file with `hitspec import curl @commands.txt`
- **Insomnia Import**: Import Insomnia export files with `hitspec import insomnia collection.json`
  - Supports Insomnia v4 export format
  - Converts variables, authentication, and request bodies
- **SSE (Server-Sent Events) Support**: Test SSE endpoints with dedicated syntax
  - Stream events with timeout configuration
  - Assert on event data, type, and count
- **API Coverage Reporting**: Measure API test coverage against OpenAPI specs
  - `--coverage` flag enables coverage tracking
  - `--openapi` flag specifies OpenAPI spec file
  - Generate HTML, JSON, or console reports with `--coverage-output`
- **Custom Annotations**: Define custom metadata with `@x-custom` or namespaced annotations like `@contract.state`
- **VSCode Extension**: Syntax highlighting and code snippets for `.http` and `.hitspec` files
  - TextMate grammar for syntax highlighting
  - Snippets for common patterns (requests, assertions, captures)

### Changed

- **Response Diff on Failure**: Console output now shows JSON diff for assertion failures in verbose mode
  - Added/removed/changed values highlighted with colors
  - Only differing paths displayed, not entire response bodies

## [1.0.1] - 2026-01-19

### Fixed

- Stress runner now extracts captures from setup requests, enabling variable chaining
- Requests with unresolved variables are skipped instead of being sent with literal `{{variable}}` strings
- Added `HasUnresolvedVariables` and `GetUnresolvedVariables` methods to env.Resolver for variable validation

## [0.1.0] - 2024-01-18

### Added

- Initial release of hitspec
- File-based HTTP API testing with `.http` and `.hitspec` files
- Variable interpolation with `{{variable}}` syntax
- Request chaining with captures
- Assertions for status, headers, body, and JSONPath
- Multiple environment support via `.env` files
- Built-in functions: `uuid()`, `timestamp()`, `random()`, and more
- Multiple output formats: console, JSON, JUnit, TAP
- Parallel test execution with `--parallel` flag
- Watch mode for automatic re-runs with `--watch` flag
- Request dependencies with `@depends` directive
- Request retry support with `@retry` directive
- Tag-based test filtering with `--tags` flag
- Name-based test filtering with `--name` flag
- Multipart form data support
- JSON Schema validation for responses
- Bail on first failure with `--bail` flag

### Commands

- `hitspec run` - Run API tests
- `hitspec validate` - Validate test file syntax
- `hitspec list` - List all tests in files
- `hitspec init` - Initialize a new project
- `hitspec version` - Show version information

[Unreleased]: https://github.com/abdul-hamid-achik/hitspec/compare/v2.24.0...HEAD
[2.24.0]: https://github.com/abdul-hamid-achik/hitspec/compare/v2.23.2...v2.24.0
[2.23.2]: https://github.com/abdul-hamid-achik/hitspec/compare/v2.23.1...v2.23.2
[2.23.1]: https://github.com/abdul-hamid-achik/hitspec/compare/v2.23.0...v2.23.1
[2.23.0]: https://github.com/abdul-hamid-achik/hitspec/compare/v2.22.0...v2.23.0
[2.22.0]: https://github.com/abdul-hamid-achik/hitspec/compare/v2.21.0...v2.22.0
[2.21.0]: https://github.com/abdul-hamid-achik/hitspec/compare/v2.20.0...v2.21.0
[2.20.0]: https://github.com/abdul-hamid-achik/hitspec/compare/v2.19.1...v2.20.0
[2.19.1]: https://github.com/abdul-hamid-achik/hitspec/compare/v2.19.0...v2.19.1
[2.19.0]: https://github.com/abdul-hamid-achik/hitspec/compare/v2.18.1...v2.19.0
[2.18.1]: https://github.com/abdul-hamid-achik/hitspec/compare/v2.18.0...v2.18.1
[2.18.0]: https://github.com/abdul-hamid-achik/hitspec/compare/v2.17.0...v2.18.0
[2.17.0]: https://github.com/abdul-hamid-achik/hitspec/compare/v2.16.0...v2.17.0
[2.16.0]: https://github.com/abdul-hamid-achik/hitspec/compare/v2.15.0...v2.16.0
[2.15.0]: https://github.com/abdul-hamid-achik/hitspec/compare/v2.14.1...v2.15.0
[2.14.1]: https://github.com/abdul-hamid-achik/hitspec/compare/v2.14.0...v2.14.1
[2.14.0]: https://github.com/abdul-hamid-achik/hitspec/compare/v2.13.0...v2.14.0
[2.13.0]: https://github.com/abdul-hamid-achik/hitspec/compare/v2.12.1...v2.13.0
[2.12.1]: https://github.com/abdul-hamid-achik/hitspec/compare/v2.12.0...v2.12.1
[2.12.0]: https://github.com/abdul-hamid-achik/hitspec/compare/v2.11.3...v2.12.0
[2.11.3]: https://github.com/abdul-hamid-achik/hitspec/compare/v2.11.2...v2.11.3
[2.11.2]: https://github.com/abdul-hamid-achik/hitspec/compare/v2.11.1...v2.11.2
[2.11.1]: https://github.com/abdul-hamid-achik/hitspec/compare/v2.11.0...v2.11.1
[2.11.0]: https://github.com/abdul-hamid-achik/hitspec/compare/v2.10.1...v2.11.0
[2.10.1]: https://github.com/abdul-hamid-achik/hitspec/compare/v2.10.0...v2.10.1
[2.10.0]: https://github.com/abdul-hamid-achik/hitspec/compare/v2.9.0...v2.10.0
[2.9.0]: https://github.com/abdul-hamid-achik/hitspec/compare/v2.8.0...v2.9.0
[2.8.0]: https://github.com/abdul-hamid-achik/hitspec/compare/v2.7.0...v2.8.0
[2.7.0]: https://github.com/abdul-hamid-achik/hitspec/compare/v2.6.0...v2.7.0
[2.6.0]: https://github.com/abdul-hamid-achik/hitspec/compare/v2.5.0...v2.6.0
[2.5.0]: https://github.com/abdul-hamid-achik/hitspec/compare/v2.4.1...v2.5.0
[2.4.1]: https://github.com/abdul-hamid-achik/hitspec/compare/v2.4.0...v2.4.1
[2.4.0]: https://github.com/abdul-hamid-achik/hitspec/compare/v2.3.0...v2.4.0
[2.3.0]: https://github.com/abdul-hamid-achik/hitspec/compare/v2.2.1...v2.3.0
[2.2.1]: https://github.com/abdul-hamid-achik/hitspec/compare/v2.2.0...v2.2.1
[2.2.0]: https://github.com/abdul-hamid-achik/hitspec/compare/v2.1.0...v2.2.0
[2.1.0]: https://github.com/abdul-hamid-achik/hitspec/compare/v2.0.2...v2.1.0
[2.0.2]: https://github.com/abdul-hamid-achik/hitspec/compare/v2.0.1...v2.0.2
[2.0.1]: https://github.com/abdul-hamid-achik/hitspec/compare/v2.0.0...v2.0.1
[2.0.0]: https://github.com/abdul-hamid-achik/hitspec/compare/v1.3.0...v2.0.0
[1.3.0]: https://github.com/abdul-hamid-achik/hitspec/compare/v1.2.3...v1.3.0
[1.2.3]: https://github.com/abdul-hamid-achik/hitspec/compare/v1.2.2...v1.2.3
[1.2.2]: https://github.com/abdul-hamid-achik/hitspec/compare/v1.2.1...v1.2.2
[1.2.1]: https://github.com/abdul-hamid-achik/hitspec/compare/v1.2.0...v1.2.1
[1.2.0]: https://github.com/abdul-hamid-achik/hitspec/compare/v1.0.1...v1.2.0
[1.0.1]: https://github.com/abdul-hamid-achik/hitspec/compare/v0.1.0...v1.0.1
[0.1.0]: https://github.com/abdul-hamid-achik/hitspec/releases/tag/v0.1.0
