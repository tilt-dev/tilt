FROM golang:1.11-alpine
WORKDIR /go/src/github.com/windmilleng/integration/watchfalse
ADD . .
RUN go install github.com/windmilleng/integration/watchfalse
ENTRYPOINT /go/bin/watchfalse
