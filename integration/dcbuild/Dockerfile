FROM golang:1.11-alpine
RUN apk add curl
WORKDIR /go/src/github.com/windmilleng/integration/dcbuild/cmd/dcbuild
ADD . .
RUN go install github.com/windmilleng/integration/dcbuild/cmd/dcbuild
ENTRYPOINT /go/bin/dcbuild
