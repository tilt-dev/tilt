#!/bin/bash
#
# Updates the Tilt docs repo with the latest version info.
#
# Usage:
# scripts/update-docs-tilt-repo.sh $VERSION
# where VERSION is of the form 0.1.0

set -euo pipefail

if [[ "${GITHUB_TOKEN-}" == "" ]]; then
    echo "Missing GITHUB_TOKEN"
    exit 1
fi

VERSION=${1//v/}
VERSION_PATTERN="^[0-9]+\.[0-9]+\.[0-9]+$"
if ! [[ $VERSION =~ $VERSION_PATTERN ]]; then
    echo "Version did not match expected pattern. Actual: $VERSION"
    exit 1
fi

DIR=$(dirname "$0")
cd "$DIR/.."

ROOT=$(mktemp -d)
git clone https://tilt-releaser:"$GITHUB_TOKEN"@github.com/tilt-dev/tilt.build "$ROOT"

set -x
go run -mod=vendor ./cmd/tilt/main.go dump cli-docs --dir="$ROOT/docs/cli"
cd "$ROOT"
sed -i -E "s/asdf install tilt .*/asdf install tilt $VERSION/" docs/install.md
sed -i -E "s/asdf global tilt .*/asdf global tilt $VERSION/" docs/install.md
sed -i -E "s/asdf install tilt .*/asdf install tilt $VERSION/" docs/upgrade.md
sed -i -E "s/asdf global tilt .*/asdf global tilt $VERSION/" docs/upgrade.md

# the sed pattern doesn't need to match the whole string.
SED_VERSION_PATTERN="[0-9]+\.[0-9]+\.[0-9]+"
sed -i -E "s|/download/v$SED_VERSION_PATTERN/tilt.$SED_VERSION_PATTERN|/download/v$VERSION/tilt.$VERSION|" docs/install.md
sed -i -E "s|/download/v$SED_VERSION_PATTERN/tilt.$SED_VERSION_PATTERN|/download/v$VERSION/tilt.$VERSION|" docs/upgrade.md
git add .

git config --global user.email "hi@tilt.dev"
git config --global user.name "Tilt Dev"
git commit -a -m "Update docs to Tilt version: $VERSION"
git push origin master

rm -fR "$ROOT"
