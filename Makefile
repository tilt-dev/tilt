.PHONY: all proto install lint test wire-check wire ensure

all: lint errcheck verify_gofmt wire-check test

SYNCLET_IMAGE := gcr.io/windmill-public-containers/tilt-synclet

proto:
	docker build -t tilt-protogen -f Dockerfile.protogen .
	docker rm tilt-protogen || exit 0
	docker run --name tilt-protogen tilt-protogen
	docker cp tilt-protogen:/go/src/github.com/windmilleng/tilt/internal/synclet/proto/synclet.pb.go internal/synclet/proto
	docker rm tilt-protogen

# Build a binary that uses synclet:latest
# TODO(nick): We should have a release build that bakes in a particular
# SYNCLET_IMAGE tag.
install:
	./hide_tbd_warning go install ./...

install-dev:
	docker build -t $(SYNCLET_IMAGE):dirty -f synclet/Dockerfile .
	$(eval HASH := $(shell docker inspect $(SYNCLET_IMAGE):dirty -f '{{.Id}}' | \
                         sed -E 's/sha256:(.{20}).*/dirty-\1/'))
	docker tag $(SYNCLET_IMAGE):dirty $(SYNCLET_IMAGE):$(HASH)
	docker push $(SYNCLET_IMAGE):$(HASH)
	./hide_tbd_warning go install -ldflags "-X './internal/synclet/sidecar.SyncletTag=$(HASH)'" ./...

lint:
	go vet -all -printfuncs=Verbosef,Infof,Debugf,PrintColorf ./...
	! grep --include=\*.go -rn . -e '^[^/].*defer [^ ]*EndPipeline(' # linting for improperly deferred EndPipeline calls; should be in closure, i.e. `defer func() { ...EndPipeline(err) }()`

build:
	./hide_tbd_warning go test -timeout 60s ./... -run nonsenseregex

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

synclet-latest:
	docker build -t $(SYNCLET_IMAGE):latest -f synclet/Dockerfile .
	docker push $(SYNCLET_IMAGE):latest

