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

# NOTE(nicks): homebrew started giving the error:
# Error: No available formula with the name "tilt".
# This env variable seems to be how people on the internet are fixing it.
# https://github.com/orgs/Homebrew/discussions/4401
export HOMEBREW_NO_INSTALL_FROM_API=1

export HOMEBREW_GITHUB_API_TOKEN="$GITHUB_TOKEN"
VERSION=${1//v/}
VERSION_PATTERN="^[0-9]+\\.[0-9]+\\.[0-9]+$"
if ! [[ $VERSION =~ $VERSION_PATTERN ]]; then
    echo "Version did not match expected pattern. Actual: $VERSION"
    exit 1
fi

# send the brew team a PR to upgrade homebrew-core
brew bump-formula-pr --no-browse --force --version="$VERSION" tilt
