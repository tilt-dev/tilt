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

touch restart.txt
kill $(cat process.txt)
