#!/bin/bash
#
# Run goreleaser in a Docker container with all the cross-compiling toolchains
# we need to do a release.

set -ex

if [[ "$GITHUB_API_TOKEN" == "" ]]; then
    echo "Missing GITHUB_API_TOKEN"
    exit 1
fi

DIR=$(dirname "$0")
cd "$DIR/.."

docker run --rm --privileged \
       -e GITHUB_TOKEN="$GITHUB_API_TOKEN" \
       -w /src/tilt \
       -v "$PWD:/src/tilt" \
       -v /var/run/docker.sock:/var/run/docker.sock \
       gcr.io/windmill-public-containers/tilt-releaser@sha256:65243c1f64435b83cd389f76a215cda62895a7b6729eec3b763e3dbcf13f7f05 \
       --rm-dist
