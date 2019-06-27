FROM busybox as builder

WORKDIR /src

RUN echo hi > hi.txt

FROM busybox

WORKDIR /dest

COPY --from=builder /src/hi.txt /dest/hi.txt

RUN cat hi.txt && exit 1
