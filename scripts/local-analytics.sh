#!/bin/bash
#
# Run a dummy local analytics server that prints each metric in a formatted JSON
# blob similar to the format it is stored in Tilt cloud.
#
# Run this script in one shell, then start Tilt like:
#
# TILT_ANALYTICS_URL=http://localhost:9988 tilt up

port=${1-9988}
running=1
interrupt() { running=; }
trap interrupt INT

echo Analytics listening on http://localhost:$port

jqscript='. as {$name, $machine, "git.origin": $gitorigin} | 
  delpaths([["name"],["machine"],["git.origin"]]) | 
  { $name, $machine, "git.origin": $gitorigin, time: (now | todate), tags: . }'

while [ "$running" ]; do
    echo -e "HTTP/1.1 200 OK\r\nContent-Length: 0\r\n\r\n" | \
        nc -l $port | tail -1 | jq "$jqscript"
done

exit 0
