.PHONY: all proto install lint test test-go test-js integration wire-check wire ensure docs check-go

check-go: lint errcheck verify_gofmt wire-check test-go
all: check-go test-js

SYNCLET_IMAGE := gcr.io/windmill-public-containers/tilt-synclet
SYNCLET_DEV_IMAGE_TAG_FILE := .synclet-dev-image-tag

scripts/protocc/protocc.py: scripts/protocc
	git submodule init
	git submodule update

proto: scripts/protocc/protocc.py
	python3 scripts/protocc/protocc.py --out go

# Build a binary that uses synclet:latest
# TODO(nick): We should have a release build that bakes in a particular
# SYNCLET_IMAGE tag.
install:
	./hide_tbd_warning go install ./...

install-dev:
	@if ! [[ -e "$(SYNCLET_DEV_IMAGE_TAG_FILE)" ]]; then echo "No dev synclet found. Run make synclet-dev."; exit 1; fi
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

build:
	./hide_tbd_warning go test -timeout 60s ./... -run nonsenseregex

test-go: 
	./hide_tbd_warning go test -timeout 60s ./...

test: test-go test-js

# skip some tests that are slow and not always relevant
shorttest:
	./hide_tbd_warning go test -tags 'skipcontainertests' -timeout 60s ./...

integration:
	./hide_tbd_warning go test -tags 'integration' -timeout 300s ./integration

dev-js:
	cd web && yarn install && yarn run start

test-js:
	cd web && yarn install && CI=true yarn test

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

ci-integration-container:
	docker build -t gcr.io/windmill-public-containers/tilt-integration-ci -f .circleci/Dockerfile.integration .circleci
	docker push gcr.io/windmill-public-containers/tilt-integration-ci

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

release:
	goreleaser --rm-dist

docs:
	docker rm tiltdocs || exit 0
	rm -fR docs/_build
	docker build -t tilt/docs -f Dockerfile.docs .
	docker run --name tiltdocs tilt/docs
	docker cp tiltdocs:/src/_build docs/
	docker rm tiltdocs

