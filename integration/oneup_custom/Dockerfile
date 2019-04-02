FROM golang:1.11-alpine
WORKDIR /go/src/github.com/windmilleng/integration/oneup_custom
ADD . .
RUN go install github.com/windmilleng/integration/oneup_custom
ENTRYPOINT /go/bin/oneup_custom
