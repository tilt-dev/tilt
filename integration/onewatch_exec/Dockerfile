FROM busybox
WORKDIR /app
ADD . .
RUN cp source.txt index.html
ENTRYPOINT busybox httpd -f -p 5000
