# clusterid

[![Build Status](https://circleci.com/gh/tilt-dev/clusterid/tree/main.svg?style=shield)](https://circleci.com/gh/tilt-dev/clusterid)
[![GoDoc](https://godoc.org/github.com/tilt-dev/clusterid?status.svg)](https://pkg.go.dev/github.com/tilt-dev/clusterid)

A small Kubernetes cluster detection library.

## Why?

The [Tilt](https://github.com/tilt-dev/tilt) project interacts with
many different types of local development clusters.

These clusters sometimes have machine-readable ways to determine
what features they support, but more often they do not.

This library uses some simple heuristics to figure out what
kind of cluster we're talking to, to figure out what features
a cluster might support.

## License

Copyright 2022 Windmill Engineering

Licensed under [the Apache License, Version 2.0](LICENSE)


