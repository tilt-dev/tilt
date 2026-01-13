#!/bin/bash
#
# Tilt Build Script
#
# Usage:
#   ./scripts/build.sh [OPTIONS]
#
# Options:
#   --quick, -q     Go binary only (skip frontend build)
#   --full, -f      Full build with frontend (default)
#   --js-only       Build frontend only
#   --no-install    Build but don't install to GOPATH
#   --clean         Clean before building
#   --help, -h      Show this help message

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"

# Defaults
BUILD_JS=true
BUILD_GO=true
DO_INSTALL=true
DO_CLEAN=false

# Colors (if terminal supports them)
if [[ -t 1 ]]; then
    RED='\033[0;31m'
    GREEN='\033[0;32m'
    YELLOW='\033[0;33m'
    BOLD='\033[1m'
    NC='\033[0m'
else
    RED='' GREEN='' YELLOW='' BOLD='' NC=''
fi

usage() {
    cat << EOF
${BOLD}Tilt Build Script${NC}

Usage: $0 [OPTIONS]

Options:
  --quick, -q     Go binary only (skip frontend build)
  --full, -f      Full build with frontend (default)
  --js-only       Build frontend only
  --no-install    Build but don't install to GOPATH
  --clean         Clean before building
  --help, -h      Show this help message

Examples:
  $0              # Full build (JS + Go binary)
  $0 --quick      # Go binary only (fastest)
  $0 -q           # Same as --quick
  $0 --js-only    # Frontend only
EOF
}

parse_args() {
    while [[ $# -gt 0 ]]; do
        case $1 in
            --quick|-q)
                BUILD_JS=false
                ;;
            --full|-f)
                BUILD_JS=true
                BUILD_GO=true
                ;;
            --js-only)
                BUILD_JS=true
                BUILD_GO=false
                DO_INSTALL=false
                ;;
            --no-install)
                DO_INSTALL=false
                ;;
            --clean)
                DO_CLEAN=true
                ;;
            --help|-h)
                usage
                exit 0
                ;;
            *)
                echo -e "${RED}Unknown option: $1${NC}"
                usage
                exit 1
                ;;
        esac
        shift
    done
}

step() {
    local num=$1 total=$2 msg=$3
    echo ""
    echo -e "${BOLD}[$num/$total] $msg${NC}"
}

check_prereqs() {
    if ! "$SCRIPT_DIR/check-prereqs.sh" > /dev/null 2>&1; then
        echo -e "${RED}Prerequisites check failed.${NC}"
        echo "Run ./scripts/check-prereqs.sh for details."
        exit 1
    fi
    echo "      All prerequisites satisfied."
}

clean_build() {
    echo "      Cleaning previous build artifacts..."
    rm -rf "$REPO_ROOT/pkg/assets/build"
    mkdir -p "$REPO_ROOT/pkg/assets/build"
    rm -f "$REPO_ROOT/bin/tilt"
}

build_js() {
    echo "      Installing dependencies..."
    cd "$REPO_ROOT/web"

    # Enable corepack if available (for yarn) - suppress download prompts
    if command -v corepack &> /dev/null; then
        COREPACK_ENABLE_DOWNLOAD_PROMPT=0 corepack enable 2>/dev/null || true
    fi

    yarn install 2>&1 | while read -r line; do
        echo "      $line"
    done

    echo "      Building React app..."
    yarn build 2>&1 | while read -r line; do
        echo "      $line"
    done

    echo "      Copying assets to pkg/assets/build/..."
    mkdir -p "$REPO_ROOT/pkg/assets/build"
    cp -r build/* "$REPO_ROOT/pkg/assets/build/"

    cd "$REPO_ROOT"
    echo -e "      ${GREEN}Frontend build complete.${NC}"
}

build_go() {
    local ldflags=""

    # Get commit SHA for version info
    local commit_sha
    commit_sha=$(git rev-parse HEAD 2>/dev/null || echo "unknown")
    ldflags="-X 'github.com/tilt-dev/tilt/internal/cli.commitSHA=$commit_sha'"

    if [[ "$DO_INSTALL" == true ]]; then
        local gopath
        gopath=$(go env GOPATH)
        echo "      Installing to $gopath/bin/tilt..."
        go install -mod vendor -ldflags "$ldflags" ./cmd/tilt/...
    else
        echo "      Building to ./bin/tilt..."
        mkdir -p "$REPO_ROOT/bin"
        go build -mod vendor -ldflags "$ldflags" -o "$REPO_ROOT/bin/tilt" ./cmd/tilt/...
    fi

    echo -e "      ${GREEN}Go build complete.${NC}"
}

main() {
    parse_args "$@"
    cd "$REPO_ROOT"

    echo -e "${BOLD}Tilt Build${NC}"
    echo "=========="

    # Calculate total steps
    local total=1  # prereqs check
    [[ "$DO_CLEAN" == true ]] && ((total++))
    [[ "$BUILD_JS" == true ]] && ((total++))
    [[ "$BUILD_GO" == true ]] && ((total++))

    local current_step=0

    # Step: Check prerequisites
    current_step=$((current_step + 1))
    step $current_step $total "Checking prerequisites"
    check_prereqs

    # Step: Clean (if requested)
    if [[ "$DO_CLEAN" == true ]]; then
        current_step=$((current_step + 1))
        step $current_step $total "Cleaning build artifacts"
        clean_build
    fi

    # Step: Build JS
    if [[ "$BUILD_JS" == true ]]; then
        current_step=$((current_step + 1))
        step $current_step $total "Building frontend (web/)"
        build_js
    fi

    # Step: Build Go
    if [[ "$BUILD_GO" == true ]]; then
        current_step=$((current_step + 1))
        step $current_step $total "Building Go binary"
        build_go
    fi

    # Summary
    echo ""
    echo -e "${GREEN}${BOLD}Build complete!${NC}"

    if [[ "$BUILD_GO" == true ]]; then
        local binary
        if [[ "$DO_INSTALL" == true ]]; then
            binary="$(go env GOPATH)/bin/tilt"
        else
            binary="$REPO_ROOT/bin/tilt"
        fi

        if [[ -f "$binary" ]]; then
            echo "  Binary: $binary"
            local version
            version=$("$binary" version 2>/dev/null || echo "unknown")
            echo "  Version: $version"
        fi
    fi

    if [[ "$BUILD_JS" == true ]] && [[ "$BUILD_GO" == false ]]; then
        echo ""
        echo "Frontend built. Run './scripts/build.sh --quick' to build Go binary."
    fi
}

main "$@"
