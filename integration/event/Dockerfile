FROM busybox

ADD . .
ENTRYPOINT echo "Starting HTTP server..." && busybox httpd -f -p 8000
