#!/usr/bin/env bash
# Print the CHANGELOG.md section of one version, without its heading, as the
# body of the GitHub release:
#
#   .github/scripts/release-notes.sh 2.21.0 [CHANGELOG.md]
#
# Fails when the section is missing or empty, so a tag cannot publish a
# release whose notes (upgrade notes included) were never written.
set -euo pipefail

version=${1:?usage: release-notes.sh <version> [changelog]}
changelog=${2:-CHANGELOG.md}

notes=$(awk -v heading="## [$version]" '
  /^## \[/ {
    if (found) exit
    if (index($0, heading) == 1) { found = 1; next }
  }
  # Skip the blank lines between the heading and the first paragraph.
  found && !started && /^[[:space:]]*$/ { next }
  found { started = 1; print }
' "$changelog")

if [ -z "$(printf '%s' "$notes" | tr -d '[:space:]')" ]; then
  echo "release-notes: $changelog has no section for $version" >&2
  exit 1
fi
printf '%s\n' "$notes"
