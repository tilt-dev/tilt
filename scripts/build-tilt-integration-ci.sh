#!/bin/bash
#
# Build a Docker CI integration container

set -ex

DIR=$(dirname "$0")
cd "$DIR/.."
 
docker buildx build --push --pull --platform linux/amd64 -t docker/tilt-integration-ci -f .circleci/Dockerfile.integration .circleci

# add some bash code to pull the image and pull out the tag
docker pull --platform linux/amd64 docker/tilt-integration-ci
DIGEST="$(docker inspect --format '{{.RepoDigests}}' docker/tilt-integration-ci | tr -d '[]')"

yq eval -i ".jobs.build-integration.docker[0].image = \"$DIGEST\"" .circleci/config.yml
