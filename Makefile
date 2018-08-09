.PHONY: all proto install lint test

all: proto

proto:
	protoc --go_out=plugins=grpc:../../../ -I. internal/proto/*.proto

install:
	go install ./...

lint:
	go vet ./...

test:
	go test ./...

ensure:
	dep ensure
