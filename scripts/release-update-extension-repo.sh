#!/bin/bash
#
# Add a new tag to the extension repo.
#
# Usage:
# scripts/release-update-extension-repo.sh $VERSION
# where VERSION is of the form v0.1.0

set -euo pipefail

if [[ "${GITHUB_TOKEN-}" == "" ]]; then
    echo "Missing GITHUB_TOKEN"
    exit 1
fi

VERSION="$1"
VERSION_PATTERN="^v[0-9]+\\.[0-9]+\\.[0-9]+$"
if ! [[ $VERSION =~ $VERSION_PATTERN ]]; then
    echo "Version did not match expected pattern. Actual: $VERSION"
    exit 1
fi

DIR=$(dirname "$0")
cd "$DIR/.."

ROOT=$(mktemp -d)
git clone https://tilt-releaser:"$GITHUB_TOKEN"@github.com/tilt-dev/tilt-extensions "$ROOT"

set -x
cd "$ROOT"
git fetch --tags
git tag -a "$VERSION" -m "$VERSION"
git push origin "$VERSION"

rm -fR "$ROOT"
