FROM golang:1.11
WORKDIR /go/src/github.com/windmilleng/integration/live_update
ADD . .
RUN go install github.com/windmilleng/integration/live_update
ENTRYPOINT /go/bin/live_update