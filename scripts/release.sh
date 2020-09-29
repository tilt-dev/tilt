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

./scripts/upload-assets.py latest --force
./scripts/goreleaser.sh

VERSION=$(git describe --abbrev=0 --tags)

docker run --rm \
       -e GITHUB_TOKEN="$GITHUB_TOKEN" \
       -w /src/tilt \
       -v "$PWD:/src/tilt:delegated" \
       --entrypoint /src/tilt/scripts/release-update-tilt-repo.sh \
       gcr.io/windmill-public-containers/tilt-releaser "$VERSION"

docker run --rm \
       -e GITHUB_TOKEN="$GITHUB_TOKEN" \
       -w /src/tilt \
       -v "$PWD:/src/tilt:delegated" \
       --entrypoint /src/tilt/scripts/release-update-tilt-docs-repo.sh \
       gcr.io/windmill-public-containers/tilt-releaser "$VERSION"

./scripts/record-release.sh "$VERSION"
