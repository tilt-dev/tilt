#!/bin/bash
#
# Tilt Development Prerequisites Checker
#
# Usage:
#   ./scripts/check-prereqs.sh
#
# Checks for required development tools and reports their status.
# Exit code 0 = all required tools present, 1 = missing required tools.

set -euo pipefail

# Version requirements
REQUIRED_GO_VERSION="1.24"
REQUIRED_NODE_VERSION="20"

# Track missing/outdated tools
MISSING=()
OUTDATED=()

# Colors (if terminal supports them)
if [[ -t 1 ]]; then
    RED='\033[0;31m'
    GREEN='\033[0;32m'
    YELLOW='\033[0;33m'
    BOLD='\033[1m'
    NC='\033[0m' # No Color
else
    RED='' GREEN='' YELLOW='' BOLD='' NC=''
fi

# Compare semantic versions: returns 0 (true) if $1 >= $2
version_gte() {
    printf '%s\n%s\n' "$2" "$1" | sort -V -C
}

print_ok() {
    local name=$1 version=$2 required=$3
    echo -e "  ${GREEN}[OK]${NC} $name $version (required: $required)"
}

print_fail() {
    local name=$1 version=$2 required=$3
    echo -e "  ${RED}[!!]${NC} $name $version (required: $required)"
}

print_warn() {
    local name=$1 note=$2
    echo -e "  ${YELLOW}[--]${NC} $name ($note)"
}

detect_os() {
    if [[ "$OSTYPE" == "linux"* ]]; then
        if [[ -f /etc/os-release ]]; then
            . /etc/os-release
            echo "Operating System: $NAME $VERSION_ID (Linux)"
        else
            echo "Operating System: Linux"
        fi
    elif [[ "$OSTYPE" == "darwin"* ]]; then
        local version
        version=$(sw_vers -productVersion 2>/dev/null || echo "unknown")
        echo "Operating System: macOS $version"
    else
        echo "Operating System: $OSTYPE"
    fi
}

check_go() {
    if ! command -v go &> /dev/null; then
        print_fail "Go" "not found" "${REQUIRED_GO_VERSION}+"
        MISSING+=("go")
        return
    fi
    local version
    version=$(go version | grep -oE 'go[0-9]+\.[0-9]+' | sed 's/go//')
    if version_gte "$version" "$REQUIRED_GO_VERSION"; then
        print_ok "Go" "$version" "${REQUIRED_GO_VERSION}+"
    else
        print_fail "Go" "$version" "${REQUIRED_GO_VERSION}+"
        OUTDATED+=("go")
    fi
}

check_node() {
    if ! command -v node &> /dev/null; then
        print_fail "Node.js" "not found" "${REQUIRED_NODE_VERSION}+"
        MISSING+=("node")
        return
    fi
    local version
    version=$(node --version | sed 's/v//' | cut -d. -f1)
    local full_version
    full_version=$(node --version)
    if [[ "$version" -ge "$REQUIRED_NODE_VERSION" ]]; then
        print_ok "Node.js" "$full_version" "${REQUIRED_NODE_VERSION}+"
    else
        print_fail "Node.js" "$full_version" "${REQUIRED_NODE_VERSION}+"
        OUTDATED+=("node")
    fi
}

check_yarn() {
    # Yarn is bundled in the project via corepack/packageManager
    # We just need corepack or yarn available
    if command -v yarn &> /dev/null; then
        local version
        version=$(yarn --version 2>/dev/null || echo "unknown")
        print_ok "Yarn" "$version" "bundled in project"
    elif command -v corepack &> /dev/null; then
        print_ok "Yarn" "via corepack" "bundled in project"
    else
        print_warn "Yarn" "not found, will use corepack"
    fi
}

check_make() {
    if ! command -v make &> /dev/null; then
        print_fail "make" "not found" "any"
        MISSING+=("make")
        return
    fi
    local version
    version=$(make --version 2>/dev/null | head -1 || echo "unknown")
    print_ok "make" "" "any"
}

check_cc() {
    local compiler=""
    local version=""

    if command -v cc &> /dev/null; then
        compiler="cc"
    elif command -v gcc &> /dev/null; then
        compiler="gcc"
    elif command -v clang &> /dev/null; then
        compiler="clang"
    fi

    if [[ -z "$compiler" ]]; then
        print_fail "C compiler" "not found" "any (gcc, clang)"
        MISSING+=("cc")
        return
    fi

    version=$($compiler --version 2>/dev/null | head -1 || echo "")
    print_ok "C compiler" "($compiler)" "any"
}

check_docker() {
    if ! command -v docker &> /dev/null; then
        print_warn "Docker" "optional, for full test suite"
        return
    fi
    local version
    version=$(docker --version 2>/dev/null | grep -oE '[0-9]+\.[0-9]+\.[0-9]+' | head -1 || echo "unknown")
    print_ok "Docker" "$version" "optional"
}

check_golangci_lint() {
    if ! command -v golangci-lint &> /dev/null; then
        print_warn "golangci-lint" "optional, for linting"
        return
    fi
    local version
    version=$(golangci-lint --version 2>/dev/null | grep -oE '[0-9]+\.[0-9]+\.[0-9]+' | head -1 || echo "unknown")
    print_ok "golangci-lint" "$version" "optional"
}

provide_install_hints() {
    echo ""
    echo -e "${BOLD}Installation Help:${NC}"

    for tool in "${MISSING[@]}" "${OUTDATED[@]}"; do
        case "$tool" in
            go)
                echo "  Go: https://golang.org/dl/ or:"
                if [[ "$OSTYPE" == "darwin"* ]]; then
                    echo "       brew install go"
                elif [[ "$OSTYPE" == "linux"* ]]; then
                    echo "       sudo apt install golang-go  # Debian/Ubuntu"
                    echo "       sudo dnf install golang     # Fedora"
                fi
                ;;
            node)
                echo "  Node.js: https://nodejs.org/ or use nvm:"
                echo "       curl -o- https://raw.githubusercontent.com/nvm-sh/nvm/v0.39.0/install.sh | bash"
                echo "       nvm install 22"
                ;;
            make)
                if [[ "$OSTYPE" == "darwin"* ]]; then
                    echo "  make: xcode-select --install"
                elif [[ "$OSTYPE" == "linux"* ]]; then
                    echo "  make: sudo apt install build-essential  # Debian/Ubuntu"
                    echo "        sudo dnf install make             # Fedora"
                fi
                ;;
            cc)
                if [[ "$OSTYPE" == "darwin"* ]]; then
                    echo "  C compiler: xcode-select --install"
                elif [[ "$OSTYPE" == "linux"* ]]; then
                    echo "  C compiler: sudo apt install build-essential  # Debian/Ubuntu"
                    echo "              sudo dnf install gcc              # Fedora"
                fi
                ;;
        esac
    done
}

main() {
    echo -e "${BOLD}Tilt Development Environment Check${NC}"
    echo "==================================="
    echo ""

    detect_os
    echo ""

    echo -e "${BOLD}Required Tools:${NC}"
    check_go
    check_node
    check_yarn
    check_make
    check_cc

    echo ""
    echo -e "${BOLD}Optional Tools:${NC}"
    check_docker
    check_golangci_lint

    if [[ ${#MISSING[@]} -gt 0 ]] || [[ ${#OUTDATED[@]} -gt 0 ]]; then
        provide_install_hints
        echo ""
        echo -e "${RED}Some required tools are missing or outdated.${NC}"
        exit 1
    fi

    echo ""
    echo -e "${GREEN}All required tools are installed and meet version requirements.${NC}"
    exit 0
}

main "$@"
