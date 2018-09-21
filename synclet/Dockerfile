# iron/go:dev is the alpine image with the go tools added
FROM iron/go:dev as builder

WORKDIR /app

ENV SRC_DIR=/go/src/github.com/windmilleng/tilt

# Add the source code
# (from current dir, add all files to dockerspace: /go/src...)
# (assumes that this is being run from $GOPATH/.../windmilleng/tilt
ADD . $SRC_DIR

# Build it
RUN cd $SRC_DIR; go build -o server ./cmd/synclet/main.go; mv server /app

# Create a minimal image with just the binary.
FROM alpine:latest

WORKDIR /app
COPY --from=builder /app/server .

ENTRYPOINT ["./server"]
