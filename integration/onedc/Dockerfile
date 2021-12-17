FROM golang:1.17-alpine
RUN apk add curl
WORKDIR /go/src/github.com/tilt-dev/integration/onedc
ADD . .
RUN go install github.com/tilt-dev/integration/onedc
ENTRYPOINT /go/bin/onedc
