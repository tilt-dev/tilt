#!/bin/bash
#
# Updates the Tilt docs repo with the latest version info.
#
# Usage:
# scripts/update-docs-tilt-repo.sh $VERSION
# where VERSION is of the form 0.1.0

set -e

if [[ "$GITHUB_TOKEN" == "" ]]; then
    echo "Missing GITHUB_TOKEN"
    exit 1
fi

VERSION=$(echo "$1" | sed 's/v//')
VERSION_PATTERN="[0-9]+\.[0-9]+\.[0-9]+"
if ! [[ $VERSION =~ $VERSION_PATTERN ]]; then
    echo "Version did not match expected pattern. Actual: $VERSION"
    exit 1
fi

DIR=$(dirname "$0")
cd "$DIR/.."

ROOT=$(mktemp -d)
git clone https://goreleaser:$GITHUB_TOKEN@github.com/tilt-dev/tilt.build $ROOT

set -x
go run ./cmd/tilt/main.go dump cli-docs --dir=$ROOT/docs/cli
cd $ROOT
sed -i -e "s/asdf install tilt .*/asdf install tilt $VERSION/" docs/install.md
sed -i -e "s/asdf global tilt .*/asdf global tilt $VERSION/" docs/install.md

# sed doesn't support + modifiers
SED_VERSION_PATTERN="[0-9]*\.[0-9]*\.[0-9]*"
sed -i -e "s|/download/v$SED_VERSION_PATTERN/tilt.$SED_VERSION_PATTERN|/download/v$VERSION/tilt.$VERSION|" docs/install.md
sed -i -e "s|/download/v$SED_VERSION_PATTERN/tilt.$SED_VERSION_PATTERN|/download/v$VERSION/tilt.$VERSION|" docs/upgrade.md
git add .
git commit -a -m "Update version numbers: $VERSION"
#git push origin master

#rm -fR $ROOT
