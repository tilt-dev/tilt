#!/bin/bash
#
# Updates the Tilt repo with the latest version info.
#
# Usage:
# scripts/release-update-tilt-repo.sh $VERSION
# where VERSION is of the form v0.1.0

set -euo pipefail

if [[ "${GITHUB_TOKEN-}" == "" ]]; then
    echo "Missing GITHUB_TOKEN"
    exit 1
fi

VERSION=${1//v/}
VERSION_PATTERN="^[0-9]+\\.[0-9]+\\.[0-9]+$"
if ! [[ $VERSION =~ $VERSION_PATTERN ]]; then
    echo "Version did not match expected pattern. Actual: $VERSION"
    exit 1
fi

DIR=$(dirname "$0")
cd "$DIR/.."

ROOT=$(mktemp -d)
git clone https://tilt-releaser:"$GITHUB_TOKEN"@github.com/tilt-dev/tilt "$ROOT"

set -x
cd "$ROOT"
sed -i -E "s/version = \".*\"/version = \"$VERSION\"/" scripts/install.ps1
sed -i -E "s/VERSION=\".*\"/VERSION=\"$VERSION\"/" scripts/install.sh
sed -i -E "s/devVersion = \".*\"/devVersion = \"$VERSION\"/" internal/cli/build.go
git add .
git commit -a -m "Update version numbers: $VERSION"
git push origin master

rm -fR "$ROOT"
