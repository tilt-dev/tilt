#!/usr/bin/env bash

set -e

DIR=$(dirname "$0")
cd "$DIR/.."

exec docker run -v "$(pwd)":/go/src/github.com/tilt-dev/tilt --workdir /go/src/github.com/tilt-dev/tilt \
   --entrypoint ./scripts/update-codegen-helper.sh \
   golang:1.15
