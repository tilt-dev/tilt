#!/usr/bin/env bash

set -e

DIR=$(dirname "$0")
cd "$DIR/.."

docker build -t tilt-protobuf-helper -f scripts/protobuf-helper.dockerfile scripts
docker run --rm -v "$(pwd)":/go/src/github.com/tilt-dev/tilt \
   --entrypoint /go/src/github.com/tilt-dev/tilt/scripts/update-protobuf-helper.sh \
   tilt-protobuf-helper

docker run --rm -e "CODEGEN_USER=$USER" -v "$(pwd)":/go/src/github.com/tilt-dev/tilt \
   --workdir /go/src/github.com/tilt-dev/tilt \
   --entrypoint ./scripts/update-codegen-helper.sh \
   golang:1.15
