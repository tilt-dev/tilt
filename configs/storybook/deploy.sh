#!/bin/bash
# Deploys the storybook helm chart

cd $(dirname $0)

set -ex

TODAY=$(date +"%Y-%m-%d")
SECONDS=$(date +"%s")
TAG="$TODAY-$SECONDS"
docker build -t gcr.io/windmill-prod/tilt-storybook:$TAG -f Dockerfile ../../
docker push gcr.io/windmill-prod/tilt-storybook:$TAG

helm upgrade --install storybook \
     --set image.repository=gcr.io/windmill-prod/tilt-storybook \
     --set image.tag=$TAG \
     ./
