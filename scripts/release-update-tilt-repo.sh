#!/bin/bash
#
# Updates the Tilt repo with the latest version info.
#
# Usage:
# scripts/update-tilt-repo.sh $VERSION
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
git clone https://goreleaser:$GITHUB_TOKEN@github.com/tilt-dev/tilt $ROOT

set -x
cd $ROOT
sed -i -e "s/version = \".*\"/version = \"$VERSION\"/" scripts/install.ps1
sed -i -e "s/VERSION=\".*\"/VERSION=\"$VERSION\"/" scripts/install.sh
sed -i -e "s/devVersion = \".*\"/devVersion = \"$VERSION\"/" internal/cli/build.go
git add .
git commit -a -m "Update version numbers: $VERSION"
git push origin master

rm -fR $ROOT
