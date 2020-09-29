#!/bin/sh

set -ex

DIR=$(realpath $(dirname $0))

go install github.com/tilt-dev/tilt/cmd/buildkitapi

cd $DIR/echo-hi-success
buildkitapi > $DIR/echo-hi-success.response.txt

cd $DIR/echo-hi-failure
buildkitapi > $DIR/echo-hi-failure.response.txt

cd $DIR/multistage-success
buildkitapi > $DIR/multistage-success.response.txt

cd $DIR/multistage-fail-run
buildkitapi > $DIR/multistage-fail-run.response.txt

cd $DIR/multistage-fail-copy
buildkitapi > $DIR/multistage-fail-copy.response.txt

cd $DIR/sleep-success
buildkitapi > $DIR/sleep-success.response.txt
buildkitapi --cache > $DIR/sleep-cache.response.txt

cd $DIR/rust-success
buildkitapi > $DIR/rust-success.response.txt
