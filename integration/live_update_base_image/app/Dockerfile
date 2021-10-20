ARG REPO
ARG CONTENT_IMAGE="live-update-base-image-content"
FROM ${REPO}/${CONTENT_IMAGE} as content-image

ARG REPO
ARG BASE_IMAGE
FROM ${REPO}/${BASE_IMAGE}

WORKDIR /app

COPY --from=content-image /usr/src/common/regular /app/message.txt

ENTRYPOINT busybox httpd -f -p 8000
