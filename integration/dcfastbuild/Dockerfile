FROM golang:1.11-alpine
RUN apk add curl

ADD . /go/src/github.com/windmilleng/tilt/integration/dcfastbuild

RUN go install github.com/windmilleng/tilt/integration/dcfastbuild/cmd/dcfastbuild