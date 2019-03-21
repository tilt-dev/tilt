FROM golang:1.11
WORKDIR /go/src/github.com/windmilleng/integration/dcbuild/cmd/dcbuild
ADD . .
RUN go install github.com/windmilleng/integration/dcbuild/cmd/dcbuild
ENTRYPOINT /go/bin/dcbuild
