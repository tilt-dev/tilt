FROM python:alpine

COPY --from=gcr.io/windmill-test-containers/live-update-base-image-common /usr/src/common/regular /app/message.txt

WORKDIR /app

ENTRYPOINT python -m http.server 8000
