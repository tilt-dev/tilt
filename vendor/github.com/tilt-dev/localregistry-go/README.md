# Local Registry Discovery

A Go implementation of the local registry discovery protocol.

[![Build Status](https://circleci.com/gh/tilt-dev/localregistry-go/tree/master.svg?style=shield)](https://circleci.com/gh/tilt-dev/localregistry-go)
[![GoDoc](https://godoc.org/github.com/tilt-dev/localregistry-go?status.svg)](https://pkg.go.dev/github.com/tilt-dev/localregistry-go)

## Background

Local clusters like Kind, K3d, Minikube, and Microk8s let users iterate on
Kubernetes quickly in a hermetic environment. To avoid network round-trip
latency, these clusters can be configured to pull from a local, insecure
registry.

[KEP 1755](https://github.com/kubernetes/enhancements/issues/1755) proposes a
standard for how these clusters should expose their support for this feature, so
that tooling can interoperate with them without redundant configuration.

## Try it

Install the `kubectl` plugin:

```
go install github.com/tilt-dev/localregistry-go/cmd/kubectl-local_registry
```

Run:

```
kubectl local-registry get
```

If your cluster explicitly advertises a local registry, this tool will print
the fields of `LocalRegistryHostingV1`.

## Use it in your tool

This repo contains library code that reads the local registry configuration
from a Kubernetes cluster, given a instance of the Go Kubernetes client.

## License

Copyright 2020 Windmill Engineering

Licensed under [the Apache License, Version 2.0](LICENSE)
