FROM busybox
WORKDIR /app
ADD . .
RUN ./compile.sh
ENTRYPOINT ./start.sh ./main.sh