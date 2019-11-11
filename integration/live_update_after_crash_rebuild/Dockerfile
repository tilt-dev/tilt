FROM busybox
WORKDIR /app
ADD . .
RUN ./compile.sh
ENTRYPOINT ./start.sh busybox httpd -f -p 8000
