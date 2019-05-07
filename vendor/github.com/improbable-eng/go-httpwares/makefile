SHELL=/bin/bash

GOFILES_NOVENDOR = $(shell go list ./... | grep -v /vendor/)

all: ensure vet fmt test

ensure:
	dep ensure

fmt:
	go fmt $(GOFILES_NOVENDOR)

vet:
	go vet $(GOFILES_NOVENDOR)

test: vet
	./scripts/test_all.sh

.PHONY: all validate test
