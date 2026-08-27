#!/usr/bin/env bash

set -euo pipefail

SCRIPT_PATH="${1:-$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/install.sh}"
TEST_ROOT=$(mktemp -d)
trap 'rm -rf "$TEST_ROOT"' EXIT

mkdir -p "$TEST_ROOT/archive" "$TEST_ROOT/mock-bin"
printf '%s\n' \
  '#!/bin/sh' \
  'case "${1:-}" in' \
  '  version) printf "%s\\n" "v0.37.7, built 2026-08-27" ;;' \
  '  verify-install) ;;' \
  'esac' > "$TEST_ROOT/archive/tilt"
chmod +x "$TEST_ROOT/archive/tilt"
tar -czf "$TEST_ROOT/tilt.tar.gz" -C "$TEST_ROOT/archive" tilt

printf '%s\n' \
  '#!/bin/sh' \
  'set -eu' \
  'if [ "${TILT_INSTALL_TEST_FAIL:-0}" = 1 ]; then' \
  '  printf "%s" "not a tar archive"' \
  'else' \
  '  cat "$TILT_INSTALL_TEST_ARCHIVE"' \
  'fi' > "$TEST_ROOT/mock-bin/curl"
chmod +x "$TEST_ROOT/mock-bin/curl"

run_install() {
  local os_type="$1"
  local fail_mode="$2"
  local run_root="$TEST_ROOT/${os_type}-${fail_mode}"
  local output="$run_root/output.log"
  local status

  mkdir -p "$run_root/home/.local/bin" "$run_root/tmp" "$run_root/work/tilt/existing"
  printf '%s\n' keep > "$run_root/work/tilt/keep"

  set +e
  (
    cd "$run_root/work"
    HOME="$run_root/home" \
      PATH="$run_root/home/.local/bin:$TEST_ROOT/mock-bin:/usr/bin:/bin" \
      TMPDIR="$run_root/tmp" \
      TILT_INSTALL_TEST_ARCHIVE="$TEST_ROOT/tilt.tar.gz" \
      TILT_INSTALL_TEST_FAIL="$fail_mode" \
      OSTYPE="$os_type" \
      bash "$SCRIPT_PATH"
  ) > "$output" 2>&1
  status=$?
  set -e

  if [ "$fail_mode" = 0 ]; then
    if [ "$status" -ne 0 ]; then
      cat "$output"
      return 1
    fi
    test -x "$run_root/home/.local/bin/tilt"
    test -f "$run_root/work/tilt/keep"
  elif [ "$status" -eq 0 ]; then
    cat "$output"
    return 1
  fi

  if find "$run_root/tmp" -mindepth 1 -maxdepth 1 -print -quit | grep -q .; then
    printf 'installer temporary directory was not cleaned: %s\n' "$run_root/tmp"
    return 1
  fi
}

run_install linux-gnu 0
run_install darwin 0
run_install linux-gnu 1
printf '%s\n' 'installer regression passed'
