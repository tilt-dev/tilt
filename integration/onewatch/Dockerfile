FROM golang:1.11-alpine
WORKDIR /go/src/github.com/windmilleng/tilt/integration/onewatch
ADD . .
RUN go install github.com/windmilleng/tilt/integration/onewatch
ENTRYPOINT ./start.sh /go/bin/onewatch