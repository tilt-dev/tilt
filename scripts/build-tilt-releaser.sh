#!/bin/bash
#
# Build a Docker container with all the cross-compiling toolchains
# we need to do a release. Pre-populate it with a Go cache.

set -ex

DIR=$(dirname "$0")
cd "$DIR/.."

docker build -t docker/tilt-releaser -f scripts/release.Dockerfile scripts
docker push docker/tilt-releaser
