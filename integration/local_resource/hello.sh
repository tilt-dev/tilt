#!/bin/bash

# 1. loops printing output every couple seconds
# 2. writes to cleanup.txt when it gets SIGTERM

set -euo pipefail

if [[ $# == 0 ]]; then
  echo "usage: $0 <msg>"
  exit 1
fi

n=1
msg="$*"

cleanup() {
  echo "cleaning up: $msg"
  echo "cleaning up: $msg" >> cleanup.txt
  exit 1
}

trap cleanup SIGTERM

while true; do
  echo "hello! $msg #$n"
  # run sleep in the background so the main thread is not blocked
  # otherwise, the signal handler doesn't run until the current sleep
  # finishes
  sleep 2&
  wait $!
  n=$((n + 1))
done
