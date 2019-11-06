FROM busybox
WORKDIR /app
ADD . .
RUN ./compile.sh
ENTRYPOINT busybox httpd -f -p 8000
