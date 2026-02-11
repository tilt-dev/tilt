#!/bin/bash
#
# Check if there are Go-related changes (anything outside web/).
# Exits 0 if Go changes detected, 1 if only web changes.
# Compares HEAD against the fork point from master.

set -euo pipefail

git fetch origin master --depth=100
MERGE_BASE=$(git merge-base origin/master HEAD)
CHANGED=$(git diff --name-only "$MERGE_BASE" HEAD)

if [ -z "$CHANGED" ]; then
  echo "No changes detected, assuming Go changes as safety net"
  exit 0
fi

# If any file is NOT under web/, there are Go-related changes
if echo "$CHANGED" | grep -qvE '^web/'; then
  echo "Go-related changes detected:"
  echo "$CHANGED" | grep -vE '^web/'
  exit 0
fi

echo "Only web/ changes detected"
exit 1
