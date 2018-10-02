.PHONY: all proto install lint test integration wire-check wire ensure

all: lint errcheck verify_gofmt wire-check test

SYNCLET_IMAGE := gcr.io/windmill-public-containers/tilt-synclet
SYNCLET_DEV_IMAGE_TAG_FILE := .synclet-dev-image-tag

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
	if ! [[ -e "$(SYNCLET_DEV_IMAGE_TAG_FILE)" ]]; then echo "No dev synclet found. Run make synclet-dev."; exit 1; fi
	./hide_tbd_warning go install -ldflags "-X 'github.com/windmilleng/tilt/internal/synclet/sidecar.SyncletTag=$$(<$(SYNCLET_DEV_IMAGE_TAG_FILE))'" ./...

define synclet-build-dev
	echo $1 > $(SYNCLET_DEV_IMAGE_TAG_FILE)
	docker tag $(SYNCLET_IMAGE):dirty $(SYNCLET_IMAGE):$1
	docker push $(SYNCLET_IMAGE):$1
endef

synclet-dev: synclet-cache
	docker build --build-arg baseImage=synclet-cache -t $(SYNCLET_IMAGE):dirty -f synclet/Dockerfile .
	$(call synclet-build-dev,$(shell docker inspect $(SYNCLET_IMAGE):dirty -f '{{.Id}}' | sed -E 's/sha256:(.{20}).*/dirty-\1/'))

build-synclet-and-install: synclet-dev install-dev

lint:
	go vet -all -printfuncs=Verbosef,Infof,Debugf,PrintColorf ./...
	! grep --include=\*.go -rn . -e '^[^/].*defer [^ ]*EndPipeline(' # linting for improperly deferred EndPipeline calls; should be in closure, i.e. `defer func() { ...EndPipeline(err) }()`

build:
	./hide_tbd_warning go test -timeout 60s ./... -run nonsenseregex

test:
	./hide_tbd_warning go test -timeout 60s ./...

integration:
	./hide_tbd_warning go test -tags 'integration' -timeout 300s ./integration

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
	docker rmi synclet-cache

synclet-cache:
	if [ "$(shell docker images synclet-cache -q)" = "" ]; then \
		docker build -t synclet-cache -f synclet/Dockerfile --target=go-cache .; \
	fi;

synclet-release:
	$(eval TAG := $(shell date +v%Y%m%d))
	docker build -t $(SYNCLET_IMAGE):$(TAG) -f synclet/Dockerfile .
	docker push $(SYNCLET_IMAGE):$(TAG)
	sed -i 's/var SyncletTag = ".*"/var SyncletTag = "$(TAG)"/' internal/synclet/sidecar/sidecar.go
