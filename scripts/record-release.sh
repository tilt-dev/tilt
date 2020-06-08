#!/bin/bash

# Notifies Tilt Cloud of a new Tilt Release

set -eu

die() {
  echo "$*" 1>&2
  exit 1
}

if [ $# -ne 1 ]; then
  die "Usage: $0 <tilt version>"
fi

if [[ ! $1 =~ ^v[0-9]+\.[0-9]+\.[0-9]+$ ]]; then
  die "error: first arg must be a version string of the form 'v#.#.#'. got '$1'."
fi

# strip the leading v, e.g., turn "v0.10.0" into "0.10.0"
VERSION="${1#v}"

URL="https://cloud.tilt.dev/api/tiltrelease/new"

TOKEN_FILE="$HOME/.windmill/token"

if [[ ! -f $TOKEN_FILE ]]; then
  die "error: $TOKEN_FILE not found. Run tilt to create one and register it to your Tilt Cloud account."
fi

TOKEN="$(cat "$TOKEN_FILE")"

JSON='{"version": "'"$VERSION"'", "timestamp": "'"$(date --iso-8601=seconds)"'"}'

HTTP_STATUS="$(curl --output /dev/stderr --write-out "%{http_code}" -sSL "$URL" -H "X-Tilt-Token: $TOKEN" -X POST -H "Content-Type: application/json" -d "$JSON")"
if [[ HTTP_STATUS -eq 401 ]]; then
  die "error: user unauthorized to record a Tilt release.

Ensure your Tilt token is registered to a Tilt Cloud account, and that Tilt Cloud account is a member of the 'Tilt Employees' Tilt Cloud team."
fi
