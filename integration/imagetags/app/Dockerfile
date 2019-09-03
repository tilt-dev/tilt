FROM busybox

COPY --from=gcr.io/windmill-test-containers/imagetags-common /usr/src/common/regular /app/message.txt

WORKDIR /app

ENTRYPOINT busybox httpd -f -p 8000
