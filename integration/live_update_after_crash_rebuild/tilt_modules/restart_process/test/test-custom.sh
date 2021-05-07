#!/bin/bash

cd $(dirname $0)

set -x
tilt ci -f custom.Tiltfile > tilt.log 2>&1
CI_EXIT=$?

tilt down

if [ $CI_EXIT -eq 0 ]; then
  echo "Expected 'tilt ci' to fail, but succeeded."
  exit 1
fi

cat tilt.log | grep -q "Are you there, pod?"
GREP_EXIT=$?

rm tilt.log

exit $GREP_EXIT
