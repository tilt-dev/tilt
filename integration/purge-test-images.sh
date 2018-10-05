#!/bin/sh

set -ex
PURGE=$(dirname $0)/purge-image.sh
$PURGE gcr.io/windmill-test-containers/integration/oneup
