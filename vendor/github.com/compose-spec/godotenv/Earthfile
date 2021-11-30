ARG GOLANG_VERSION=1.17.1
ARG ALPINE_VERSION=3.14

FROM golang:${GOLANG_VERSION}-alpine${ALPINE_VERSION}
WORKDIR /code

code:
    FROM +base
    COPY . .

golangci:
    ARG GOLANGCI_VERSION=v1.40.1
    FROM golangci/golangci-lint:${GOLANGCI_VERSION}-alpine
    SAVE ARTIFACT /usr/bin/golangci-lint

lint:
    FROM +code
    COPY +golangci/golangci-lint /usr/bin/golangci-lint
    RUN golangci-lint run --timeout 5m ./...

test:
    FROM +code
    RUN go test ./...

all:
    BUILD +lint
    BUILD +test
