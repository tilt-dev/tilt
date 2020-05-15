FROM golang:1.11-alpine
RUN apk add curl

ADD . /go/src/github.com/tilt-dev/tilt/integration/dcfastbuild

RUN go install github.com/tilt-dev/tilt/integration/dcfastbuild/cmd/dcfastbuild

ENTRYPOINT /go/bin/dcfastbuild
