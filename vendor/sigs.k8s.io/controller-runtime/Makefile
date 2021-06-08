#!/usr/bin/env bash

#  Copyright 2020 The Kubernetes Authors.
#
#  Licensed under the Apache License, Version 2.0 (the "License");
#  you may not use this file except in compliance with the License.
#  You may obtain a copy of the License at
#
#      http://www.apache.org/licenses/LICENSE-2.0
#
#  Unless required by applicable law or agreed to in writing, software
#  distributed under the License is distributed on an "AS IS" BASIS,
#  WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
#  See the License for the specific language governing permissions and
#  limitations under the License.

# If you update this file, please follow
# https://suva.sh/posts/well-documented-makefiles

## --------------------------------------
## General
## --------------------------------------

SHELL:=/usr/bin/env bash
.DEFAULT_GOAL:=help

# Use GOPROXY environment variable if set
GOPROXY := $(shell go env GOPROXY)
ifeq ($(GOPROXY),)
GOPROXY := https://proxy.golang.org
endif
export GOPROXY

# Active module mode, as we use go modules to manage dependencies
export GO111MODULE=on

# Tools.
TOOLS_DIR := hack/tools
TOOLS_BIN_DIR := $(TOOLS_DIR)/bin
GOLANGCI_LINT := $(abspath $(TOOLS_BIN_DIR)/golangci-lint)
GO_APIDIFF := $(TOOLS_BIN_DIR)/go-apidiff
CONTROLLER_GEN := $(TOOLS_BIN_DIR)/controller-gen

# The help will print out all targets with their descriptions organized bellow their categories. The categories are represented by `##@` and the target descriptions by `##`.
# The awk commands is responsible to read the entire set of makefiles included in this invocation, looking for lines of the file as xyz: ## something, and then pretty-format the target and help. Then, if there's a line with ##@ something, that gets pretty-printed as a category.
# More info over the usage of ANSI control characters for terminal formatting: https://en.wikipedia.org/wiki/ANSI_escape_code#SGR_parameters
# More info over awk command: http://linuxcommand.org/lc3_adv_awk.php
.PHONY: help
help:  ## Display this help
	@awk 'BEGIN {FS = ":.*##"; printf "\nUsage:\n  make \033[36m<target>\033[0m\n"} /^[a-zA-Z_0-9-]+:.*?##/ { printf "  \033[36m%-15s\033[0m %s\n", $$1, $$2 } /^##@/ { printf "\n\033[1m%s\033[0m\n", substr($$0, 5) } ' $(MAKEFILE_LIST)

## --------------------------------------
## Testing
## --------------------------------------

.PHONY: test
test: test-tools ## Run the script check-everything.sh which will check all.
	TRACE=1 ./hack/check-everything.sh

.PHONY: test-tools
test-tools: ## tests the tools codebase (setup-envtest)
	cd tools/setup-envtest && go test ./...

## --------------------------------------
## Binaries
## --------------------------------------

$(GOLANGCI_LINT): $(TOOLS_DIR)/go.mod # Build golangci-lint from tools folder.
	cd $(TOOLS_DIR) && go build -tags=tools -o bin/golangci-lint github.com/golangci/golangci-lint/cmd/golangci-lint

$(GO_APIDIFF): $(TOOLS_DIR)/go.mod # Build go-apidiff from tools folder.
	cd $(TOOLS_DIR) && go build -tags=tools -o bin/go-apidiff github.com/joelanford/go-apidiff

$(CONTROLLER_GEN): $(TOOLS_DIR)/go.mod # Build controller-gen from tools folder.
	cd $(TOOLS_DIR) && go build -tags=tools -o bin/controller-gen sigs.k8s.io/controller-tools/cmd/controller-gen

## --------------------------------------
## Linting
## --------------------------------------

.PHONY: lint-libs
lint-libs: $(GOLANGCI_LINT) ## Lint library codebase.
	$(GOLANGCI_LINT) run -v

.PHONY: lint-tools
lint-tools: $(GOLANGCI_LINT) ## Lint tools codebase.
	cd tools/setup-envtest && $(GOLANGCI_LINT) run -v

.PHONY: lint
lint: lint-libs lint-tools

## --------------------------------------
## Generate
## --------------------------------------

.PHONY: modules
modules: ## Runs go mod to ensure modules are up to date.
	go mod tidy
	cd $(TOOLS_DIR); go mod tidy

.PHONY: generate
generate: $(CONTROLLER_GEN) ## Runs controller-gen for internal types for config file
	$(CONTROLLER_GEN) object paths="./pkg/config/v1alpha1/...;./examples/configfile/custom/v1alpha1/..."

## --------------------------------------
## Cleanup / Verification
## --------------------------------------

.PHONY: clean
clean: ## Cleanup.
	$(MAKE) clean-bin

.PHONY: clean-bin
clean-bin: ## Remove all generated binaries.
	rm -rf hack/tools/bin

.PHONY: verify-modules
verify-modules: modules
	@if !(git diff --quiet HEAD -- go.sum go.mod); then \
		echo "go module files are out of date, please run 'make modules'"; exit 1; \
	fi
