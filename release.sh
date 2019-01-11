#!/usr/bin/env bash

set -euo pipefail

if [[ $# != 2 ]]; then
  echo "Bumps the current release version to the current dev version."
  echo "Builds the a new release and publishes it to github."
  echo "Usage: $0 <new dev version number> <release tag message>"
  echo
  echo "The current dev version number is: $(<dev_version)"
  exit 1
fi

RELEASE=$(<dev_version)
NEW_DEV_VERSION=$1
TAG_MESSAGE="$2"

echo "Publishing $RELEASE and bumping dev version to $NEW_DEV_VERSION."

TAG="v$RELEASE"

git fetch --tags
git tag -a "$TAG" -m "$TAG_MESSAGE"
git push origin "$TAG"
make release

echo "$RELEASE published."
mv dev_version release_version
echo "$(date -u +"%Y-%m-%d")" > release_date
echo "$NEW_DEV_VERSION" > dev_version
echo "Rebuilding docs."
make docs

echo "Docs and version files have been updated locally:"
git status

echo
echo "You now need to commit and make a PR."
