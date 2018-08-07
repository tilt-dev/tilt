.PHONY: all proto

all: proto

proto:
	protoc --go_out=plugins=grpc:../../../ -I. internal/proto/*.proto

install:
	go install ./...
