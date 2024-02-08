#!/usr/bin/env bash

set -e

DIR=$(dirname "$0")
cd "$DIR/.."


# docker mounts don't work in our CI setup - just run the scripts directly
if [[ $CI == true ]]; then
  # TODO - get this working in CI
  # scripts/update-protobuf-helper.sh
  scripts/update-codegen-helper.sh
  exit 0
fi

docker build --load -t tilt-protobuf-helper -f scripts/protobuf-helper.dockerfile scripts
docker run --rm -v "$(pwd)":/go/src/github.com/tilt-dev/tilt \
   --entrypoint /go/src/github.com/tilt-dev/tilt/scripts/update-protobuf-helper.sh \
   tilt-protobuf-helper

docker run --rm -e "CODEGEN_USER=$USER" -v "$(pwd)":/go/src/github.com/tilt-dev/tilt \
   --workdir /go/src/github.com/tilt-dev/tilt \
   --entrypoint ./scripts/update-codegen-helper.sh \
   golang:1.21
