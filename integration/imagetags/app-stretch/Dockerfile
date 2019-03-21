FROM python:alpine

COPY --from=gcr.io/windmill-test-containers/imagetags-common:stretch /usr/src/common/stretch /app/message.txt

WORKDIR /app

ENTRYPOINT python -m http.server 8000
