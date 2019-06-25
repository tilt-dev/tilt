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

while true; do
    rm -f restart.txt
    
    $* &
    process_id=$!
    echo "$process_id" > process.txt
    wait $process_id
    EXIT_CODE=$?
    if [ ! -f restart.txt ]; then
        exit $EXIT_CODE
    fi
    echo "Restarting"
done
