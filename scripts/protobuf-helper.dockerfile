FROM golang:1.18

ENV GOPATH=/go
WORKDIR /go/src
RUN apt update && apt install -y protobuf-compiler
RUN go install k8s.io/code-generator/cmd/go-to-protobuf@v0.20.2
RUN go install github.com/gogo/protobuf/protoc-gen-gogo@latest
RUN go install golang.org/x/tools/cmd/goimports@latest
