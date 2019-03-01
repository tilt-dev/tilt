#!/bin/bash
#
# Usage:
# scripts/upload-assets.sh VERSION
#
# where VERSION is a version string like "1.2.3"
#
# Generates static assets for HTML, JS, and CSS.
# Then uploads them to the public bucket.

VERSION="$1"
if [[ "$VERSION" == "" ]]; then
    echo "Usage: scripts/upload-assets.sh VERSION"
    exit 1
fi

set -ex
cd web
yarn run build
gsutil cp -r build "gs://tilt-static-assets/v$VERSION"
