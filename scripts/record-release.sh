#!/bin/sh

# Notifies Tilt Cloud of a new Tilt Release

set -eu

if [ $# -ne 1 ]; then
  echo "Usage: $0 <tilt version>"
  exit 1
fi

# strip the leading v, e.g., turn "v0.10.0" into "0.10.0"
VERSION="${1#v}"

URL="https://cloud.tilt.dev/api/tiltrelease/new"

TOKEN="$(cat ~/.windmill/token)"

JSON='{"version": "'"$VERSION"'", "timestamp": "'"$(date --iso-8601=seconds)"'"}'

curl -sSfL "$URL" -H "X-Tilt-Token: $TOKEN" -X POST -H "Content-Type: application/json" -d "$JSON"
