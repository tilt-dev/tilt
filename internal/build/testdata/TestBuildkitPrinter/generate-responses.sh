#!/bin/sh

set -ex

DIR=$(realpath $(dirname $0))

go install github.com/windmilleng/tilt/cmd/buildkitapi

cd $DIR/echo-hi-success
buildkitapi > $DIR/echo-hi-success.response.txt

cd $DIR/echo-hi-failure
buildkitapi > $DIR/echo-hi-failure.response.txt
