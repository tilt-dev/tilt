#!/bin/bash

# Updates cloud.tilt.dev with the new release.

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

DIR=$(dirname "$0")
cd "$DIR/.."

ROOT=$(mktemp -d)
git clone https://tilt-releaser:"$GITHUB_TOKEN"@github.com/tilt-dev/cloud.tilt.dev "$ROOT"
echo "{\"Found\":false,\"Username\":\"\",\"TeamName\":\"\",\"TeamRole\":\"\",\"SuggestedTiltVersion\":\"$VERSION\",\"UserID\":0}" > "$ROOT/web/api/whoami"

git config --global user.email "it@tilt.dev"
git config --global user.name "Tilt Dev"
git commit -a -m "Notify all tilt users of new version: $VERSION"
git push origin main

rm -fR "$ROOT"
