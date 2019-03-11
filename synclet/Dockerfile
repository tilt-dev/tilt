ARG baseImage="golang/1.12-alpine"

FROM ${baseImage} as builder

WORKDIR /go/src/github.com/windmilleng/tilt

# Add the source code
# (from current dir, add all files to dockerspace: /go/src...)
# (assumes that this is being run from $GOPATH/.../windmilleng/tilt
ADD . .

RUN mkdir -p /app

# Build the server binary.
RUN go build -o server ./cmd/synclet/main.go
RUN mv server /app/

# Build an image with just the Go cache artifacts
FROM golang/1.12-alpine as go-cache

COPY --from=builder /root/.cache /root/.cache

# Create a minimal image with just the binary.
FROM alpine:latest

WORKDIR /app
COPY --from=builder /app/server .

ENTRYPOINT ["./server"]
