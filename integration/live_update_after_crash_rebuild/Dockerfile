FROM busybox
WORKDIR /app
ADD . .
RUN ./compile.sh
ENTRYPOINT ./main.sh
