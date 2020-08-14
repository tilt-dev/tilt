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
./scripts/record-release.sh "$$(git describe --abbrev=0 --tags)"
