#!/bin/bash

# Test case for https://github.com/tilt-dev/tilt-extensions/issues/92
#
# This job will always exit with a non-zero status code; make sure
# that docker_build_with_restart surfaces this error code to k8s,
# so k8s knows that the job failed. (Thus, we expect the `tilt ci`
# call to fail.)
cd $(dirname $0)

set -x
tilt ci > tilt.log 2>&1
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
