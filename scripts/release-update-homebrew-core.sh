#!/bin/bash
#
# Updates the Tilt repo with the latest version info.
#
# Usage:
# scripts/release-update-homebrew-core.sh $VERSION
# where VERSION is of the form v0.1.0

set -euo pipefail

if [[ "${GITHUB_TOKEN-}" == "" ]]; then
    echo "Missing GITHUB_TOKEN"
    exit 1
fi

export HOMEBREW_GITHUB_API_TOKEN="$GITHUB_TOKEN"
VERSION=${1//v/}
VERSION_PATTERN="^[0-9]+\\.[0-9]+\\.[0-9]+$"
if ! [[ $VERSION =~ $VERSION_PATTERN ]]; then
    echo "Version did not match expected pattern. Actual: $VERSION"
    exit 1
fi

git config --global user.email "it@tilt.dev"
git config --global user.name "Tilt Dev"

# send the brew team a PR to upgrade homebrew-core
brew bump-formula-pr --version="$VERSION" tilt
