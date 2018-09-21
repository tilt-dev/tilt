.PHONY: all proto install lint test wire-check wire ensure

all: lint errcheck verify_gofmt wire-check test

proto:
	docker build -t tilt-protogen -f Dockerfile.protogen .
	docker rm tilt-protogen || exit 0
	docker run --name tilt-protogen tilt-protogen
	docker cp tilt-protogen:/go/src/github.com/windmilleng/tilt/internal/synclet/proto/synclet.pb.go internal/synclet/proto
	docker rm tilt-protogen

install:
	./hide_tbd_warning go install ./...

lint:
	go vet -all -printfuncs=Verbosef,Infof,Debugf,PrintColorf ./...
	! grep --include=\*.go -rn . -e '^[^/].*defer [^ ]*EndPipeline(' # linting for improperly deferred EndPipeline calls; should be in closure, i.e. `defer func() { ...EndPipeline(err) }()`

test:
	./hide_tbd_warning go test -timeout 60s ./...

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

timing: install
	./scripts/timing.py

wire:
	wire ./internal/engine
	wire ./internal/cli
	wire ./internal/synclet

wire-check:
	wire check ./internal/engine
	wire check ./internal/cli
	wire check ./internal/synclet

ci-container:
	docker build -t gcr.io/windmill-public-containers/tilt-ci -f .circleci/Dockerfile .circleci
	docker push gcr.io/windmill-public-containers/tilt-ci

clean:
	go clean -cache -testcache -r -i ./...
