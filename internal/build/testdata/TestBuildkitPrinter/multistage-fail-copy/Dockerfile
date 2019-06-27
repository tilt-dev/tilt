FROM busybox as builder

WORKDIR /src

RUN echo hi > hi.txt

FROM busybox

WORKDIR /dest

COPY --from=builder /dest/hi.txt /src/hi.txt

RUN cat hi.txt
