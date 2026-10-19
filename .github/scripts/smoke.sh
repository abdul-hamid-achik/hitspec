#!/usr/bin/env bash
# Binary-level smoke test: runs the real hitspec binary and looks at what it
# prints. A silent `hitspec run` shipped for six releases because no test did.
#
#   .github/scripts/smoke.sh /path/to/hitspec
#
# 1. hitspec init -> validate -> mock (background) -> run in a temp project;
#    `run` must exit 0 and print a report containing "passed".
# 2. Every examples/*/*.http that runs offline (each carries >>>mock blocks)
#    is served by `hitspec mock` and run against it, same assertions. The
#    examples all point at http://localhost:3000; copies are rewritten to a
#    private port so a busy port 3000 cannot break the job.
#
# SMOKE_REQUIRE_SQLITE=1 turns a missing sqlite3 CLI (needed by the database
# example) from a skip into a failure.
set -euo pipefail

BIN=${1:?usage: smoke.sh /path/to/hitspec}
BIN=$(cd "$(dirname "$BIN")" && pwd)/$(basename "$BIN")
ROOT=$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)

WORK=$(mktemp -d)
export HOME="$WORK/home"
mkdir -p "$HOME"
MOCK_PID=""

cleanup() {
  stop_mock
  rm -rf "$WORK"
}
trap cleanup EXIT

fail() {
  echo "SMOKE FAIL: $*" >&2
  exit 1
}

stop_mock() {
  if [ -n "$MOCK_PID" ]; then
    kill "$MOCK_PID" 2>/dev/null || true
    wait "$MOCK_PID" 2>/dev/null || true
    MOCK_PID=""
  fi
}

PORT=39100

# start_mock <file> <dir>: serve <file> on the next free port, wait until up.
start_mock() {
  PORT=$((PORT + 1))
  (cd "$2" && exec "$BIN" mock "$1" --port "$PORT") >"$WORK/mock.log" 2>&1 &
  MOCK_PID=$!
  for _ in $(seq 1 50); do
    if curl -s -o /dev/null "http://127.0.0.1:$PORT/"; then
      return 0
    fi
    kill -0 "$MOCK_PID" 2>/dev/null || { cat "$WORK/mock.log" >&2; fail "mock server exited early"; }
    sleep 0.2
  done
  cat "$WORK/mock.log" >&2
  fail "mock server did not come up on port $PORT"
}

# check_run <label> <dir> <args...>: run `hitspec run`, assert exit 0 and a
# non-empty stdout report that says "passed".
check_run() {
  local label=$1 dir=$2
  shift 2
  local out="$WORK/stdout.txt" err="$WORK/stderr.txt" code=0
  (cd "$dir" && "$BIN" run "$@" --no-color) >"$out" 2>"$err" || code=$?
  if [ "$code" -ne 0 ]; then
    cat "$out" >&2
    cat "$err" >&2
    fail "$label: hitspec run exited $code"
  fi
  [ -s "$out" ] || { cat "$err" >&2; fail "$label: hitspec run printed nothing on stdout"; }
  grep -q "passed" "$out" || { cat "$out" >&2; fail "$label: stdout has no \"passed\" summary"; }
  echo "ok   $label ($(grep -E 'passed' "$out" | tail -1 | sed 's/^ *//'))"
}

echo "== init / validate / mock / run"
PROJ="$WORK/proj"
mkdir -p "$PROJ"
(cd "$PROJ" && "$BIN" init >/dev/null)
[ -f "$PROJ/hitspec.yaml" ] && [ -f "$PROJ/example.http" ] || fail "init did not create hitspec.yaml and example.http"
(cd "$PROJ" && "$BIN" validate example.http) || fail "validate rejected the scaffold"
# The scaffold points at port 3000; move it to a private port.
PORT=39100
sed "s#localhost:3000#localhost:$((PORT + 1))#" "$PROJ/hitspec.yaml" >"$PROJ/hitspec.yaml.new"
mv "$PROJ/hitspec.yaml.new" "$PROJ/hitspec.yaml"
start_mock example.http "$PROJ"
check_run "init scaffold" "$PROJ" example.http
stop_mock

echo "== examples/"
count=0
for dir in "$ROOT"/examples/*/; do
  name=$(basename "$dir")
  for src in "$dir"*.http; do
    [ -e "$src" ] || continue
    file=$(basename "$src")
    case "$name" in
      database)
        if ! command -v sqlite3 >/dev/null 2>&1; then
          [ "${SMOKE_REQUIRE_SQLITE:-0}" = "1" ] && fail "sqlite3 is required for examples/database"
          echo "skip $name/$file (no sqlite3)"
          continue
        fi
        ;;
    esac
    copy="$WORK/ex-$name"
    rm -rf "$copy"
    cp -R "$dir" "$copy"
    next=$((PORT + 1))
    sed "s#localhost:3000#localhost:$next#g" "$copy/$file" >"$copy/$file.new"
    mv "$copy/$file.new" "$copy/$file"

    "$BIN" validate "$copy/$file" >/dev/null || fail "$name/$file does not validate"
    extra=()
    if [ "$name" = "database" ]; then
      (cd "$copy" && sqlite3 test.db <seed.sql)
      extra=(--allow-db)
    fi
    start_mock "$file" "$copy"
    check_run "$name/$file" "$copy" "$file" ${extra[@]+"${extra[@]}"}
    if [ "$name" = "stress-test" ]; then
      stress_out="$WORK/stress.txt"
      (cd "$copy" && "$BIN" run "$file" --stress --duration 2s --rate 5 --no-color) >"$stress_out" 2>&1 \
        || { cat "$stress_out" >&2; fail "$name: stress run failed"; }
      grep -q "STRESS TEST SUMMARY" "$stress_out" || { cat "$stress_out" >&2; fail "$name: no stress summary printed"; }
      echo "ok   $name/$file --stress"
    fi
    stop_mock
    count=$((count + 1))
  done
done
[ "$count" -ge 1 ] || fail "no examples were run"
echo "smoke: all good ($count example files)"
