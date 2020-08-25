#!/bin/bash
#
# Build a Docker container with all the cross-compiling toolchains
# we need to do a release. Pre-populate it with a Go cache.

set -ex

DIR=$(dirname "$0")
cd "$DIR/.."

docker build -t tilt-releaser-base -f scripts/release.Dockerfile scripts
docker container rm tilt-releaser-go-cache || true

# We deliberately don't give this container any API keys so it can't
# accidentally publish.
docker run --name tilt-releaser-go-cache --privileged \
       -w /src/tilt \
       -v "$PWD:/src/tilt:delegated" \
       -v /var/run/docker.sock:/var/run/docker.sock \
       tilt-releaser-base \
       --rm-dist --skip-validate --snapshot --skip-publish

# Commit all the Go cache artifacts we generated
docker commit tilt-releaser-go-cache gcr.io/windmill-public-containers/tilt-releaser
docker push gcr.io/windmill-public-containers/tilt-releaser
docker container rm tilt-releaser-go-cache
