#! /bin/sh

echo "Are you there, pod?"
sleep 1

# Exit with a non-zero status code; we check that docker_build_with_restart
# surfaces this error code to k8s, so k8s knows that the job failed.
echo "Exiting with status code 123 😱"
exit 123
