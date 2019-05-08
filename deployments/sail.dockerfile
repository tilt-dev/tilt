FROM golang:1.12
WORKDIR /go/src/github.com/windmilleng/tilt

# Build the Go code
ADD . .
RUN make install-sail

ENTRYPOINT sail --web-mode=prod
