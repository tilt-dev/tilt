.PHONY: all proto install lint test

all: lint errcheck test verify_gofmt

proto:
	docker build -t tilt-protogen -f Dockerfile.protogen .
	docker rm tilt-protogen || exit 0
	docker run --name tilt-protogen tilt-protogen
	docker cp tilt-protogen:/go/src/github.com/windmilleng/tilt/internal/proto/daemon.pb.go internal/proto/
	docker rm tilt-protogen

install:
	go install ./...

lint:
	go tool vet -all -printfuncs=Verbosef,Infof,Debugf $$(go list -f {{.Dir}} ./... | grep -v /vendor/)

test:
	go test -timeout 60s ./...

ensure:
	dep ensure

verify_gofmt:
	bash -c 'diff <(go fmt ./...) <(echo -n)'

benchmark:
	go test -run=XXX -bench=. ./...

errcheck:
	errcheck -ignoretests -ignoregenerated ./...

start_tracer:
	docker run -d -p5775:5775/udp -p6831:6831/udp -p6832:6832/udp -p5778:5778 -p16686:16686 -p14268:14268 -p9411:9411 jaegertracing/all-in-one:0.8.0

timing:
	./scripts/timing.py
