#!/bin/bash
#
# Do a complete release. Run on CI.
# Upload assets, run goreleaser, and notify Tilt Cloud of the new release binaries.

set -ex

if [[ "$(which brew)" == "" ]]; then
    echo "Missing Homebrew"
    exit 1
fi

if [[ "$GITHUB_TOKEN" == "" ]]; then
    echo "Missing GITHUB_TOKEN"
    exit 1
fi

if [[ "$DOCKER_TOKEN" == "" ]]; then
    echo "Missing DOCKER_TOKEN"
    exit 1
fi

if [[ "$TILT_CLOUD_TOKEN" == "" ]]; then
    echo "Missing Tilt release token"
    exit 1
fi

DIR=$(dirname "$0")
cd "$DIR/.."

echo "$DOCKER_TOKEN" | docker login --username "$DOCKER_USERNAME" --password-stdin

mkdir -p ~/.windmill
echo "$TILT_CLOUD_TOKEN" > ~/.windmill/token

git fetch --tags
git config --global user.email "it@tilt.dev"
git config --global user.name "Tilt Dev"

VERSION=$(git describe --abbrev=0 --tags)

goreleaser --clean

./scripts/release-update-tilt-repo.sh "$VERSION"
./scripts/release-update-tilt-docs-repo.sh "$VERSION"
./scripts/record-release.sh "$VERSION"
./scripts/release-update-homebrew-core.sh "$VERSION"
./scripts/release-update-extension-repo.sh "$VERSION"
