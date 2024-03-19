FROM alpine
RUN apk add busybox-extras curl
WORKDIR /app
ADD . .
ENTRYPOINT httpd -f -p 8000
