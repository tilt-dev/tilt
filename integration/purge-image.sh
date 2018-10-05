#!/bin/sh

set -ex
IMAGE="$1"
TAGS=$(gcloud container images list-tags $IMAGE --limit=999999 --format='get(digest)')
for digest in $TAGS; do
    gcloud container images delete -q --force-delete-tags "${IMAGE}@${digest}"
done
