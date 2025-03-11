.PHONY: all install lint test test-go check-js test-js test-storybook integration wire-check wire ensure goimports vendor shellcheck release-container update-codegen update-codegen-go update-codegen-starlark update-codegen-ts

all: check-js test-js test-storybook

# There are 2 Go bugs that cause problems on CI:
# 1) Linker memory usage blew up in Go 1.11
# 2) Go incorrectly detects the number of CPUs when running in containers,
#    and sets the number of parallel jobs to the number of CPUs.
# This makes CI blow up frequently without out-of-memory errors.
# Manually setting the number of parallel jobs helps fix this.
# https://github.com/golang/go/issues/26186#issuecomment-435544512
GO_PARALLEL_JOBS := 4

CIRCLECI := $(if $(CIRCLECI),$(CIRCLECI),false)

GOIMPORTS_LOCAL_ARG := -local github.com/tilt-dev

# Build a binary the current commit SHA
install:
	go install -mod vendor -ldflags "-X 'github.com/tilt-dev/tilt/internal/cli.commitSHA=$$(git merge-base master HEAD)'" ./cmd/tilt/...

# disable optimizations and inlining, to allow more complete information when attaching a debugger or capturing a profile
install-debug:
	go install -mod vendor -gcflags "all=-N -l" ./...

lint: golangci-lint

lintfix:
	LINT_FLAGS=--fix make golangci-lint

build:
	go test -mod vendor -p $(GO_PARALLEL_JOBS) -timeout 60s ./... -run nonsenseregex

test-go:
ifneq ($(CIRCLECI),true)
		gotestsum -- -mod vendor -p $(GO_PARALLEL_JOBS) -timeout 100s ./...
else
		mkdir -p test-results
		gotestsum --format standard-quiet --junitfile test-results/unit-tests.xml -- ./... -mod vendor -p $(GO_PARALLEL_JOBS) -timeout 100s
endif

test: test-go test-js

# skip some tests that are slow and not always relevant
# TODO(matt) skiplargetiltfiletests only skips the tiltfile DC+Helm tests at the moment
# we might also want to skip the ones in engine
shorttest:
	go test -mod vendor -p $(GO_PARALLEL_JOBS) -short -tags skipcontainertests,skiplargetiltfiletests -timeout 100s ./...

shorttestsum:
ifneq ($(CIRCLECI),true)
	gotestsum -- -mod vendor -p $(GO_PARALLEL_JOBS) -short -tags skipcontainertests,skiplargetiltfiletests -timeout 100s ./...
else
	mkdir -p test-results
	gotestsum --format standard-quiet --junitfile test-results/unit-tests.xml --rerun-fails=2 --rerun-fails-max-failures=10 --packages="./..." -- -mod vendor -count 1 -p $(GO_PARALLEL_JOBS) -short -tags skipcontainertests,skiplargetiltfiletests -timeout 100s
endif

integration:
ifneq ($(CIRCLECI),true)
		go test -mod vendor -v -count 1 -p $(GO_PARALLEL_JOBS) -tags 'integration' -timeout 30m ./integration
else
		mkdir -p test-results
		gotestsum --format dots --junitfile test-results/unit-tests.xml -- ./integration -mod vendor -count 1 -p $(GO_PARALLEL_JOBS) -tags 'integration' -timeout 1000s
endif

# Run the integration tests on kind
integration-kind:
	KIND_CLUSTER_NAME=integration ./integration/kind-with-registry.sh
	KUBECONFIG="$(kind get kubeconfig-path --name="integration")" go test -mod vendor -p $(GO_PARALLEL_JOBS) -tags 'integration' -timeout 30m ./integration -count 1
	kind delete cluster --name=integration

# Run the extension integration tests against the current kubecontext
test-extensions:
	scripts/test-extensions.sh

dev-js:
	cd web && yarn install && yarn run start

check-js:
	cd web && yarn install --immutable
	# make sure there are no compilation errors or lint warnings
	cd web && CI=true yarn build
	cd web && yarn run check

build-js:
	cd web && yarn install --immutable
	cd web && yarn build
	cp -r web/build/* pkg/assets/build

test-js:
	cd web && yarn install --immutable
ifneq ($(CIRCLECI),true)
	cd web && CI=true yarn test
else
	cd web && CI=true yarn ci
endif

test-storybook:
	cd web && yarn start-storybook --ci --smoke-test

goimports:
	goimports -w -l $(GOIMPORTS_LOCAL_ARG) cmd/ integration/ internal/ pkg/

benchmark:
	go test -mod vendor -run=XXX -bench=. ./...

golangci-lint:
ifneq ($(CIRCLECI),true)
	GOFLAGS="-mod=vendor" golangci-lint run $(LINT_FLAGS) -v --timeout 300s
else
	mkdir -p test-results
	GOFLAGS="-mod=vendor" golangci-lint run -v --timeout 300s --out-format junit-xml > test-results/lint.xml
endif

wire:
	wire ./internal/engine ./internal/engine/buildcontrol ./internal/cli
	goimports -w -l $(GOIMPORTS_LOCAL_ARG) internal/

wire-check:
	wire check ./internal/engine ./internal/engine/buildcontrol ./internal/cli

release-container:
	scripts/build-tilt-releaser.sh

ci-container:
	docker buildx build --push --pull --platform linux/amd64 -t docker/tilt-ci -f .circleci/Dockerfile .circleci

ci-integration-container:
	docker buildx build --push --pull --platform linux/amd64 -t docker/tilt-integration-ci -f .circleci/Dockerfile.integration .circleci

clean:
	go clean -cache -testcache -r -i ./...

prettier:
	cd web && yarn install
	cd web && yarn prettier

storybook:
	cd web && yarn install
	cd web && yarn storybook

tilt-toast-container:
	docker build --platform linux/amd64 -t docker/tilt-toast -f Dockerfile.toast .circleci
	docker push docker/tilt-toast

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

update-codegen: update-codegen-go update-codegen-ts update-codegen-starlark

update-codegen-go:
	scripts/update-codegen.sh
	goimports -w -l $(GOIMPORTS_LOCAL_ARG) pkg

update-codegen-starlark:
	go install github.com/tilt-dev/tilt-starlark-codegen@latest
	tilt-starlark-codegen ./pkg/apis/core/v1alpha1 ./internal/tiltfile/v1alpha1
	goimports -w -l $(GOIMPORTS_LOCAL_ARG) internal/

update-codegen-ts:
	toast proto-ts

release-build:
	toast -f build.toast.yml
