FROM golang:1.11
WORKDIR /go/src/github.com/windmilleng/integration/onedc
ADD . .
RUN go install github.com/windmilleng/integration/onedc
ENTRYPOINT /go/bin/onedc