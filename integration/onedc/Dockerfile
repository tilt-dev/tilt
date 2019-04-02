FROM golang:1.11-alpine
RUN apk add curl
WORKDIR /go/src/github.com/windmilleng/integration/onedc
ADD . .
RUN go install github.com/windmilleng/integration/onedc
ENTRYPOINT /go/bin/onedc