#!/bin/bash
#
# Do a complete release.
# Upload assets, run goreleaser, and notify Tilt Cloud of the new release binaries.

set -ex

if [[ "$GITHUB_TOKEN" == "" ]]; then
    echo "Missing GITHUB_TOKEN"
    exit 1
fi

DIR=$(dirname "$0")
cd "$DIR/.."

./scripts/upload-assets.py latest
./scripts/goreleaser.sh

VERSION=$(git describe --abbrev=0 --tags)
./scripts/release-update-tilt-repo.sh "$VERSION"
./scripts/release-update-tilt-docs-repo.sh "$VERSION"
./scripts/record-release.sh "$VERSION"
