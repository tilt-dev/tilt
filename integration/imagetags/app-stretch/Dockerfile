FROM busybox

COPY --from=gcr.io/windmill-test-containers/imagetags-common:stretch /usr/src/common/stretch /app/message.txt

WORKDIR /app

ENTRYPOINT busybox httpd -f -p 8000
