#!/bin/sh
#
# A helper script to implement restart_container when the docker runtime isn't available.
#
# Usage:
#   Copy start.sh and restart.sh to your container working dir.
#
#   Make your container entrypoint:
#   ./start.sh path-to-binary [args]
#
#   To restart the container:
#   ./restart.sh

set -euo pipefail

process_id=""

trap quit TERM INT

quit() {
  if [ -n "$process_id" ]; then
    kill $process_id
  fi
}

while true; do
    rm -f restart.txt

    "$@" &
    process_id=$!
    echo "$process_id" > process.txt
    set +e
    wait $process_id
    EXIT_CODE=$?
    set -e
    if [ ! -f restart.txt ]; then
        exit $EXIT_CODE
    fi
    echo "Restarting"
done
