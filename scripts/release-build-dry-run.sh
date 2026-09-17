#!/bin/bash
#
# Dry-run of a release build: compiles the release binaries locally with the
# same cross-compiling toolchain CI uses, but only as a snapshot (nothing is
# tagged, pushed, or published). Outputs to ./dist.
#
# Usage:
#   scripts/release-build-dry-run.sh [extra goreleaser args...]
#
# Environment:
#   GOOS, GOARCH - restrict the build to a single OS/arch
#   GR_ARGS      - extra goreleaser args (same as passing them on the CLI)
#
# To test a single target:
#   GOOS=linux GOARCH=arm64 scripts/release-build-dry-run.sh --id tilt-linux-arm64 --single-target

set -euo pipefail

# keep image in sync with .circleci/config.yml
IMAGE="tiltdev/tilt-releaser@sha256:d0953d17c76f318249ff05ebeffc47da8c0f3ba07dbcf2197b3e76503d764c1c"

# The container is amd64-only, and the toolchains inside it cross-compile
# from amd64. On other hosts (e.g., Apple Silicon), this runs emulated.
PLATFORM="linux/amd64"

# Matches the GOPATH layout inside the release image.
WORKDIR="/go/src/github.com/tilt-dev/tilt"

DIR=$(dirname "$0")
cd "$DIR/.."

GOOS="${GOOS:-}"
GOARCH="${GOARCH:-}"
GR_ARGS="${GR_ARGS:-}"

# Cache the Go build cache across runs, so repeat builds aren't from scratch.
CACHE_DIR=~/.cache/tilt/release/go-build
mkdir -p "$CACHE_DIR"

docker run --rm \
       --platform "$PLATFORM" \
       -e GOOS="$GOOS" \
       -e GOARCH="$GOARCH" \
       -w "$WORKDIR" \
       -v "$CACHE_DIR:/root/.cache/go-build" \
       -v "$PWD:$WORKDIR:delegated" \
       "$IMAGE" \
       bash -c "set -euo pipefail
                make build-js
                goreleaser --verbose build --snapshot --clean $GR_ARGS $*"

# The container runs as root. On Linux, that leaves root-owned build output
# on the host, so hand it back to the invoking user.
if [[ "$(uname)" == "Linux" && "$(id -u)" != "0" ]]; then
    docker run --rm \
           --platform "$PLATFORM" \
           -w "$WORKDIR" \
           -v "$PWD:$WORKDIR:delegated" \
           "$IMAGE" \
           chown -R "$(id -u):$(id -g)" dist web/build pkg/assets/build
fi
