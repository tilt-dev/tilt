.PHONY: all proto install lint test test-go check-js test-js test-storybook integration wire-check wire ensure check-go goimports proto-webview proto-webview-ts vendor shellcheck release-container update-codegen

all: check-go check-js test-js test-storybook

# There are 2 Go bugs that cause problems on CI:
# 1) Linker memory usage blew up in Go 1.11
# 2) Go incorrectly detects the number of CPUs when running in containers,
#    and sets the number of parallel jobs to the number of CPUs.
# This makes CI blow up frequently without out-of-memory errors.
# Manually setting the number of parallel jobs helps fix this.
# https://github.com/golang/go/issues/26186#issuecomment-435544512
GO_PARALLEL_JOBS := 4

CIRCLECI := $(if $(CIRCLECI),$(CIRCLECI),false)

GOIMPORTS_LOCAL_ARG := -local github.com/tilt-dev/tilt

proto:
	toast proto-ts

# Build a binary the current commit SHA
install:
	go install -mod vendor -ldflags "-X 'github.com/tilt-dev/tilt/internal/cli.commitSHA=$$(git merge-base master HEAD)'" ./cmd/tilt/...

# disable optimizations and inlining, to allow more complete information when attaching a debugger or capturing a profile
install-debug:
	go install -mod vendor -gcflags "all=-N -l" ./...

lint: golangci-lint

build:
	go test -mod vendor -p $(GO_PARALLEL_JOBS) -timeout 60s ./... -run nonsenseregex

test-go:
ifneq ($(CIRCLECI),true)
		go test -mod vendor -p $(GO_PARALLEL_JOBS) -timeout 100s ./...
else
		mkdir -p test-results
		gotestsum --format standard-quiet --junitfile test-results/unit-tests.xml -- ./... -mod vendor -p $(GO_PARALLEL_JOBS) -timeout 100s
endif

test-go-helm-only:
ifneq ($(CIRCLECI),true)
		go test -mod vendor -p $(GO_PARALLEL_JOBS) -timeout 100s ./internal/tiltfile -run "(?i)(.*)Helm(.*)"
else
		mkdir -p test-results
		gotestsum --format standard-quiet --junitfile test-results/unit-tests.xml -- ./internal/tiltfile -mod vendor -p $(GO_PARALLEL_JOBS) -timeout 100s -run "(?i)(.*)Helm(.*)"
endif

test: test-go test-js

# skip some tests that are slow and not always relevant
# TODO(matt) skiplargetiltfiletests only skips the tiltfile DC+Helm tests at the moment
# we might also want to skip the ones in engine
shorttest:
	go test -mod vendor -p $(GO_PARALLEL_JOBS) -tags skipcontainertests,skiplargetiltfiletests -timeout 100s ./...

shorttestsum:
ifneq ($(CIRCLECI),true)
	gotestsum -- -mod vendor -p $(GO_PARALLEL_JOBS) -tags skipcontainertests,skiplargetiltfiletests -timeout 100s ./...
else
	mkdir -p test-results
	gotestsum --format standard-quiet --junitfile test-results/unit-tests.xml -- ./... -mod vendor -count 1 -p $(GO_PARALLEL_JOBS) -tags skipcontainertests,skiplargetiltfiletests -timeout 100s
endif

integration:
ifneq ($(CIRCLECI),true)
		go test -mod vendor -v -count 1 -p $(GO_PARALLEL_JOBS) -tags 'integration' -timeout 700s ./integration
else
		mkdir -p test-results
		gotestsum --format dots --junitfile test-results/unit-tests.xml -- ./integration -mod vendor -count 1 -p $(GO_PARALLEL_JOBS) -tags 'integration' -timeout 700s
endif

# Run the integration tests on kind
integration-kind:
	KIND_CLUSTER_NAME=integration ./integration/kind-with-registry.sh
	KUBECONFIG="$(kind get kubeconfig-path --name="integration")" go test -mod vendor -p $(GO_PARALLEL_JOBS) -tags 'integration' -timeout 700s ./integration -count 1
	kind delete cluster --name=integration

# Run the extension integration tests against the current kubecontext
test-extensions:
	scripts/test-extensions.sh

dev-js:
	cd web && yarn install && yarn run start

check-js:
	cd web && yarn install --frozen-lockfile
	# make sure there are no compilation errors or lint warnings
	cd web && CI=true yarn build
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

test-storybook:
	cd web && yarn start-storybook --ci --smoke-test

goimports:
	goimports -w -l $(GOIMPORTS_LOCAL_ARG) internal
	goimports -w -l $(GOIMPORTS_LOCAL_ARG) pkg
	goimports -w -l $(GOIMPORTS_LOCAL_ARG) cmd

benchmark:
	go test -mod vendor -run=XXX -bench=. ./...

golangci-lint:
ifneq ($(CIRCLECI),true)
	GOFLAGS="-mod=vendor" golangci-lint run -v --timeout 120s
else
	mkdir -p test-results
	GOFLAGS="-mod=vendor" golangci-lint run -v --timeout 120s --out-format junit-xml > test-results/lint.xml
endif

wire:
	toast wire

wire-dev:
	wire ./internal/engine && wire ./internal/engine/buildcontrol && wire ./internal/cli

wire-check:
	wire check ./internal/engine
	wire check ./internal/cli

release-container:
	scripts/build-tilt-releaser.sh

ci-container:
	docker build -t gcr.io/windmill-public-containers/tilt-ci -f .circleci/Dockerfile .circleci
	docker push gcr.io/windmill-public-containers/tilt-ci

ci-integration-container:
	docker build -t gcr.io/windmill-public-containers/tilt-integration-ci -f .circleci/Dockerfile.integration .circleci
	docker push gcr.io/windmill-public-containers/tilt-integration-ci

clean:
	go clean -cache -testcache -r -i ./...

release:
	./scripts/release.sh

prettier:
	cd web && yarn install
	cd web && yarn prettier

storybook:
	cd web && yarn install
	cd web && yarn storybook

tilt-toast-container:
	docker build -t gcr.io/windmill-public-containers/tilt-toast -f Dockerfile.toast .circleci
	docker push gcr.io/windmill-public-containers/tilt-toast

ensure: vendor

vendor:
	go mod vendor
	go mod tidy

cli-docs:
	rm -fR ../tilt.build/docs/cli
	mkdir ../tilt.build/docs/cli
	tilt dump cli-docs --dir=../tilt.build/docs/cli

test_install_version_check: install
	NO_INSTALL=1 PATH="~/go/bin:$$PATH" scripts/install.sh

shellcheck:
	find ./scripts -type f -name '*.sh' -exec docker run --rm -it -e SHELLCHECK_OPTS="-e SC2001" -v $$(pwd):/mnt nlknguyen/alpine-shellcheck {} \;

update-codegen:
	scripts/update-codegen.sh
	goimports -w -l $(GOIMPORTS_LOCAL_ARG) pkg
