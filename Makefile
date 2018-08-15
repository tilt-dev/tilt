.PHONY: all proto install lint test

all: lint darwin linux test verify_gofmt

proto:
	docker build -t tilt-protogen -f Dockerfile.protogen .
	docker rm tilt-protogen || exit 0
	docker run --name tilt-protogen tilt-protogen
	docker cp tilt-protogen:/go/src/github.com/windmilleng/tilt/internal/proto/daemon.pb.go internal/proto/
	docker rm tilt-protogen

install:
	go install ./...

lint:
	go vet ./...

test:
	go test ./...

ensure:
	dep ensure

darwin:
	GOOS=darwin go build ./...

linux:
	GOOS=linux go build ./...

verify_gofmt:
	bash -c 'diff <(go fmt ./...) <(echo -n)'
