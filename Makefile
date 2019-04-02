.PHONY: all proto install lint test test-go check-js test-js integration wire-check wire ensure check-go

check-go: lint errcheck verify_gofmt wire-check test-go
all: check-go check-js test-js

# There are 2 Go bugs that cause problems on CI:
# 1) Linker memory usage blew up in Go 1.11
# 2) Go incorrectly detects the number of CPUs when running in containers,
#    and sets the number of parallel jobs to the number of CPUs.
# This makes CI blow up frequently without out-of-memory errors.
# Manually setting the number of parallel jobs helps fix this.
# https://github.com/golang/go/issues/26186#issuecomment-435544512
GO_PARALLEL_JOBS := 8

SYNCLET_IMAGE := gcr.io/windmill-public-containers/tilt-synclet
SYNCLET_DEV_IMAGE_TAG_FILE := .synclet-dev-image-tag

CIRCLECI := $(if $(CIRCLECI),$(CIRCLECI),false)

scripts/protocc/protocc.py: scripts/protocc
	git submodule init
	git submodule update

proto: scripts/protocc/protocc.py
	python3 scripts/protocc/protocc.py --out go

# Build a binary that uses synclet:latest
# TODO(nick): We should have a release build that bakes in a particular
# SYNCLET_IMAGE tag.
install:
	go install ./...

install-dev:
	@if ! [[ -e "$(SYNCLET_DEV_IMAGE_TAG_FILE)" ]]; then echo "No dev synclet found. Run make synclet-dev."; exit 1; fi
	go install -ldflags "-X 'github.com/windmilleng/tilt/internal/synclet/sidecar.SyncletTag=$$(<$(SYNCLET_DEV_IMAGE_TAG_FILE))'" ./...

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
	go test -p $(GO_PARALLEL_JOBS) -timeout 60s ./... -run nonsenseregex

test-go:
ifneq ($(CIRCLECI),true)
		go test -p $(GO_PARALLEL_JOBS) -timeout 80s ./...
else
		mkdir -p test-results
		gotestsum --format standard-quiet --junitfile test-results/unit-tests.xml -- ./... -p $(GO_PARALLEL_JOBS) -timeout 80s
endif

test: test-go test-js

# skip some tests that are slow and not always relevant
shorttest:
	go test -p $(GO_PARALLEL_JOBS) -tags 'skipcontainertests' -timeout 60s ./...

integration:
ifneq ($(CIRCLECI),true)
		go test -p $(GO_PARALLEL_JOBS) -tags 'integration' -timeout 500s ./integration
else
		mkdir -p test-results
		gotestsum --format standard-quiet --junitfile test-results/unit-tests.xml -- ./integration -p $(GO_PARALLEL_JOBS) -tags 'integration' -timeout 500s
endif

dev-js:
	cd web && yarn install && yarn run start

check-js:
	cd web && yarn install
	cd web && yarn run check

test-js:
	cd web && yarn install
	cd web && CI=true yarn test

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

prettier:
	cd web && yarn install
	cd web && yarn run prettier --write "src/**/*.ts*"

