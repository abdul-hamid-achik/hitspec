#!/usr/bin/env bash
# gosec gate shared by CI and `task security:gate`.
#
#   .github/scripts/gosec.sh [report.json]
#
# 1. Writes a full report (medium severity and up) to report.json when a path
#    is given. Informational: the ~80 medium findings are mostly G301/G302/
#    G304/G306 (file permissions and user-supplied paths), which are inherent
#    to a CLI that reads and writes the user's own .http files.
# 2. Gates on HIGH severity findings with at least MEDIUM confidence, after the
#    triaged exclusions below. A new high-severity finding fails the build.
#
# GOSEC=/path/to/gosec overrides the binary.
set -euo pipefail

GOSEC=${GOSEC:-gosec}
REPORT=${1:-}

# Triage of the high-severity findings present when the gate was introduced
# (gosec v2.29.0). Every entry is a deliberate decision, not a silenced alarm:
#
#  G404 (math/rand): used for weighted request selection in the stress
#       scheduler and for the $random()/$randomInt() style built-ins. Neither
#       is security-sensitive; crypto/rand would only add cost.
#  G402 packages/http/client.go: InsecureSkipVerify is set only when the user
#       passes --insecure/-k or sets validateSSL: false, which is the documented
#       way to test against self-signed endpoints.
#  G118 (goroutine with context.Background): graceful-shutdown goroutines in
#       serve, mock and the recording proxy must outlive the already-cancelled
#       request/run context to drain connections.
#  G703 packages/serve/handler_files.go, packages/clientmgr/files.go: taint
#       analysis flags the request path, which both files resolve inside the
#       workspace (isPathWithin / Manager path checks) and restrict to
#       .http/.hitspec before touching the disk. Re-check this exclusion
#       whenever the path validation changes (symlink handling in particular).
EXCLUDE_RULES='packages/http/client\.go:G402;packages/serve/server\.go:G118;packages/proxy/recorder\.go:G118;packages/mock/server\.go:G118;packages/serve/handler_files\.go:G703;packages/clientmgr/files\.go:G703'

if [ -n "$REPORT" ]; then
  "$GOSEC" -severity=medium -confidence=medium -fmt=json -out="$REPORT" -quiet ./... || true
fi

exec "$GOSEC" -severity=high -confidence=medium -exclude=G404 \
  -exclude-rules="$EXCLUDE_RULES" -quiet ./...
