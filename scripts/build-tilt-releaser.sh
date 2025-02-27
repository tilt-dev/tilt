#!/bin/bash
#
# Build a Docker container with all the cross-compiling toolchains
# we need to do a release. Pre-populate it with a Go cache.

set -ex

DIR=$(dirname "$0")
cd "$DIR/.."

docker buildx build --push -t docker/tilt-releaser -f scripts/release.Dockerfile scripts

# add some bash code to pull the image and pull out the tag
docker pull docker/tilt-releaser
DIGEST="$(docker inspect --format '{{.RepoDigests}}' docker/tilt-releaser | tr -d '[]')"

yq eval -i ".jobs.release-dry-run.docker[0].image = \"$DIGEST\"" .circleci/config.yml
yq eval -i ".jobs.release.docker[0].image = \"$DIGEST\"" .circleci/config.yml
yq eval -i ".image = \"$DIGEST\"" build.toast.yml
