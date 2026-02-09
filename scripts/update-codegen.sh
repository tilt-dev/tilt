#!/usr/bin/env bash

set -e

DIR=$(dirname "$0")
cd "$DIR/.."


# docker mounts don't work in our CI setup - just run the scripts directly
if [[ $CI == true ]]; then
  # TODO - get this working in CI
  # scripts/update-protobuf-helper.sh

  export CODEGEN_UID=$(id -u)
  export CODEGEN_GID=$(id -g)
  scripts/update-codegen-helper.sh
  exit 0
fi

docker run --rm -e "CODEGEN_UID=$(id -u)" -e "CODEGEN_GID=$(id -g)" -v "$(pwd)":/go/src/github.com/tilt-dev/tilt \
   --workdir /go/src/github.com/tilt-dev/tilt \
   --entrypoint ./scripts/update-codegen-helper.sh \
   golang:1.25
