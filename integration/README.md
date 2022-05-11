# Tilt Integration Tests

Runs the Tilt binary and ensures that services are correctly deployed.

Who You Are: A developer who has `kubectl` configured to talk to an existing cluster.

What This Framework Does: Compiles the Tilt binary, deploys servers to the
`tilt-integration` namespace, and cleans up when it finishes.

NOTE: The `tilt-integration` namespace will NOT be deleted afterwards.

Each subdirectory is a test case driven by the file of the same name
(e.g., `oneup_.go` drives the test driven by the data in `oneup`).

Run the tests with

```
go test -tags 'integration' -timeout 30m ./integration
```

or

```
make integration
```

These tests will not run with the normal `make test`.

On CircleCI, we run these tests against a clean Kubernetes cluster using [kind](https://kind.sigs.k8s.io/).
To create a single-use kind cluster locally and run the tests against it, run:
```
make integration-kind
```
