#!/bin/bash
#
# Build a Docker CI container

set -ex

DIR=$(dirname "$0")
cd "$DIR/.."
 
docker buildx build --push --pull --platform linux/amd64 -t docker/tilt-ci -f .circleci/Dockerfile .circleci

# add some bash code to pull the image and pull out the tag
docker pull docker/tilt-ci
DIGEST="$(docker inspect --format '{{.RepoDigests}}' docker/tilt-ci | tr -d '[]')"

yq eval -i ".jobs.build-linux.docker[0].image = \"$DIGEST\"" .circleci/config.yml
yq eval -i ".jobs.check-docs.docker[0].image = \"$DIGEST\"" .circleci/config.yml
