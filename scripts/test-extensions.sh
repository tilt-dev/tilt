#!/bin/sh
#
# Checkout extensions to a temporary directory and run tilt against them.

set -ex

DIR=$(mktemp -d)
git clone https://github.com/tilt-dev/tilt-extensions "$DIR"
cd "$DIR"
export TILT_WEB_MODE="prod"
timeout 1000s ./test.sh
