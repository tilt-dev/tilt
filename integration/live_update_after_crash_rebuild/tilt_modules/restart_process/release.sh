#!/bin/bash

set -ex

TIMESTAMP=$(date +'%Y-%m-%d')
IMAGE_NAME='tiltdev/restart-helper'
IMAGE_WITH_TAG=$IMAGE_NAME:$TIMESTAMP

# build binary for tilt-restart-wrapper
env GOOS=linux GOARCH=amd64 go build tilt-restart-wrapper.go

# build Docker image with static binaries of:
# - tilt-restart-wrapper (compiled above)
# - entr (dependency of tilt-restart-wrapper)
docker build . -t $IMAGE_NAME
docker push $IMAGE_NAME

docker tag $IMAGE_NAME $IMAGE_WITH_TAG
docker push $IMAGE_WITH_TAG

echo "Successfully built and pushed $IMAGE_WITH_TAG"



