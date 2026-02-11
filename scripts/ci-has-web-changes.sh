#!/bin/bash
#
# Check if there are web-related changes.
# Exits 0 if web changes detected, 1 if only Go changes.
# Compares HEAD against the fork point from master.

set -euo pipefail

git fetch origin master --depth=100
MERGE_BASE=$(git merge-base origin/master HEAD)
CHANGED=$(git diff --name-only "$MERGE_BASE" HEAD)

if [ -z "$CHANGED" ]; then
  echo "No changes detected, assuming web changes as safety net"
  exit 0
fi

# web/, Makefile (has web build targets), .circleci/ (affects everything)
if echo "$CHANGED" | grep -qE '^(web/|Makefile$|\.circleci/)'; then
  echo "Web-related changes detected:"
  echo "$CHANGED" | grep -E '^(web/|Makefile$|\.circleci/)'
  exit 0
fi

echo "No web-related changes detected"
exit 1
