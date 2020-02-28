.PHONY: all proto install lint test test-go check-js test-js integration wire-check wire ensure check-go goimports proto-webview proto-webview-ts vendor shellcheck

all: check-go check-js test-js

# There are 2 Go bugs that cause problems on CI:
# 1) Linker memory usage blew up in Go 1.11
# 2) Go incorrectly detects the number of CPUs when running in containers,
#    and sets the number of parallel jobs to the number of CPUs.
# This makes CI blow up frequently without out-of-memory errors.
# Manually setting the number of parallel jobs helps fix this.
# https://github.com/golang/go/issues/26186#issuecomment-435544512
GO_PARALLEL_JOBS := 4

SYNCLET_IMAGE := gcr.io/windmill-public-containers/tilt-synclet
SYNCLET_DEV_IMAGE_TAG_FILE := .synclet-dev-image-tag

CIRCLECI := $(if $(CIRCLECI),$(CIRCLECI),false)

GOIMPORTS_LOCAL_ARG := -local github.com/windmilleng/tilt

proto:
	toast synclet-proto
	toast proto-ts

# Build a binary that uses the synclet tag specified in sidecar.go
install:
	go install -mod vendor -ldflags "-X 'github.com/windmilleng/tilt/internal/cli.commitSHA=$$(git merge-base master HEAD)'" ./cmd/tilt/...

# Build a binary that uses a dev synclet image produced by `make synclet-dev`
install-dev:
	@if ! [[ -e "$(SYNCLET_DEV_IMAGE_TAG_FILE)" ]]; then echo "No dev synclet found. Run make synclet-dev."; exit 1; fi
	go install -mod vendor -ldflags "-X 'github.com/windmilleng/tilt/internal/synclet/sidecar.SyncletTag=$$(<$(SYNCLET_DEV_IMAGE_TAG_FILE))'" ./...

# disable optimizations and inlining, to allow more complete information when attaching a debugger or capturing a profile
install-debug:
	go install -mod vendor -gcflags "all=-N -l" ./...

define synclet-build-dev
	echo $1 > $(SYNCLET_DEV_IMAGE_TAG_FILE)
	docker tag $(SYNCLET_IMAGE):dirty $(SYNCLET_IMAGE):$1
	docker push $(SYNCLET_IMAGE):$1
endef

synclet-dev: synclet-cache
	docker build --build-arg baseImage=synclet-cache -t $(SYNCLET_IMAGE):dirty -f synclet/Dockerfile .
	$(call synclet-build-dev,$(shell docker inspect $(SYNCLET_IMAGE):dirty -f '{{.Id}}' | sed -E 's/sha256:(.{20}).*/dirty-\1/'))

build-synclet-and-install: synclet-dev install-dev

lint: golangci-lint

build:
	go test -mod vendor -p $(GO_PARALLEL_JOBS) -timeout 60s ./... -run nonsenseregex

test-go:
ifneq ($(CIRCLECI),true)
		go test -mod vendor -p $(GO_PARALLEL_JOBS) -timeout 80s ./...
else
		mkdir -p test-results
		gotestsum --format standard-quiet --junitfile test-results/unit-tests.xml -- ./... -mod vendor -p $(GO_PARALLEL_JOBS) -timeout 80s
endif

test-go-helm-only:
ifneq ($(CIRCLECI),true)
		go test -mod vendor -p $(GO_PARALLEL_JOBS) -timeout 80s ./internal/tiltfile -run "(?i)(.*)Helm(.*)"
else
		mkdir -p test-results
		gotestsum --format standard-quiet --junitfile test-results/unit-tests.xml -- ./internal/tiltfile -mod vendor -p $(GO_PARALLEL_JOBS) -timeout 80s -run "(?i)(.*)Helm(.*)"
endif

test: test-go test-js

# skip some tests that are slow and not always relevant
shorttest:
	# TODO(matt) skipdockercomposetests only skips the tiltfile DC tests at the moment
	# we might also want to skip the ones in engine
	go test -mod vendor -p $(GO_PARALLEL_JOBS) -tags skipcontainertests,skipdockercomposetests -timeout 60s ./...

shorttestsum:
	gotestsum -- -mod vendor -p $(GO_PARALLEL_JOBS) -tags skipcontainertests,skipdockercomposetests -timeout 60s ./...

integration:
ifneq ($(CIRCLECI),true)
		go test -mod vendor -v -count 1 -p $(GO_PARALLEL_JOBS) -tags 'integration' -timeout 700s ./integration
else
		mkdir -p test-results
		gotestsum --format standard-quiet --junitfile test-results/unit-tests.xml -- ./integration -mod vendor -count 1 -p $(GO_PARALLEL_JOBS) -tags 'integration' -timeout 700s
endif

# Run the integration tests on kind
integration-kind:
	KIND_CLUSTER_NAME=integration ./integration/kind-with-registry.sh
	KUBECONFIG="$(kind get kubeconfig-path --name="integration")" go test -mod vendor -p $(GO_PARALLEL_JOBS) -tags 'integration' -timeout 700s ./integration -count 1
	kind delete cluster --name=integration

dev-js:
	cd web && yarn install && yarn run start

check-js:
	cd web && yarn install --frozen-lockfile
	cd web && yarn run check

build-js:
	cd web && yarn install --frozen-lockfile
	cd web && yarn build

test-js:
	cd web && yarn install --frozen-lockfile
ifneq ($(CIRCLECI),true)
	cd web && CI=true yarn test
else
	cd web && CI=true yarn ci
endif

goimports:
	goimports -w -l $(GOIMPORTS_LOCAL_ARG) $$(go list -f {{.Dir}} ./...)

benchmark:
	go test -mod vendor -run=XXX -bench=. ./...

golangci-lint:
ifneq ($(CIRCLECI),true)
	GOFLAGS="-mod=vendor" golangci-lint run -v --timeout 90s
else
	mkdir -p test-results
	GOFLAGS="-mod=vendor" golangci-lint run -v --timeout 90s --out-format junit-xml > test-results/lint.xml
endif

wire:
	toast wire

wire-dev:
	wire ./internal/engine && wire ./internal/cli && wire ./internal/synclet

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

custom-synclet-release:
	$(eval TAG := $(if $(SYNCLET_TAG),$(SYNCLET_TAG),$(shell date +v%Y%m%d)))
	docker build -t $(SYNCLET_IMAGE):$(TAG) -f synclet/Dockerfile .
	docker push $(SYNCLET_IMAGE):$(TAG)

release:
	goreleaser --rm-dist

prettier:
	cd web && yarn install
	cd web && yarn run prettier --write "src/**/*.ts*"

storybook:
	cd web && yarn install
	cd web && yarn storybook

tilt-toast-container:
	docker build -t gcr.io/windmill-public-containers/tilt-toast -f Dockerfile.toast .circleci
	docker push gcr.io/windmill-public-containers/tilt-toast

ensure: vendor

vendor:
	go mod vendor

cli-docs:
	rm -fR ../tilt.build/docs/cli
	mkdir ../tilt.build/docs/cli
	tilt dump cli-docs --dir=../tilt.build/docs/cli

test_install_version_check: install
	NO_INSTALL=1 PATH="~/go/bin:$$PATH" scripts/install.sh

shellcheck:
	find ./scripts -type f -name '*.sh' -exec docker run --rm -it -v $$(pwd):/mnt nlknguyen/alpine-shellcheck {} \;
