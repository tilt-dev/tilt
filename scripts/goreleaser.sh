#!/bin/bash
#
# Run goreleaser in a Docker container with all the cross-compiling toolchains
# we need to do a release.

set -ex

if [[ "$GITHUB_TOKEN" == "" ]]; then
    echo "Missing GITHUB_TOKEN"
    exit 1
fi

DIR=$(dirname "$0")
cd "$DIR/.."

docker login
docker run --rm --privileged \
       -e GITHUB_TOKEN="$GITHUB_TOKEN" \
       -w /src/tilt \
       -v ~/.docker:/root/.docker \
       -v "$PWD:/src/tilt" \
       -v /var/run/docker.sock:/var/run/docker.sock \
       gcr.io/windmill-public-containers/tilt-releaser@sha256:d033db8b2180aef7dcdefb28bd094aa6f162dbe182c923d84c7e61826c49e33f \
       --rm-dist
