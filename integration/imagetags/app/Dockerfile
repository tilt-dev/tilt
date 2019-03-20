FROM python:alpine

COPY --from=gcr.io/windmill-test-containers/imagetags-common /usr/src/common/regular /app/message.txt

WORKDIR /app

ENTRYPOINT python -m http.server 8000
