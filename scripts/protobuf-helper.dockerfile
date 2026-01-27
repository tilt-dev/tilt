FROM golang:1.25

ENV GOPATH=/go
WORKDIR /go/src
RUN apt update && apt install -y protobuf-compiler
RUN go install k8s.io/code-generator/cmd/go-to-protobuf@v0.24.0
RUN go install github.com/gogo/protobuf/protoc-gen-gogo@latest
RUN go install golang.org/x/tools/cmd/goimports@latest
