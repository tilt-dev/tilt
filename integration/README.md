# Tilt Integration Tests

Runs the Tilt binary and ensures that services are correctly deployed.

Who You Are: A developer who has kubectl configured to talk to an existing cluster.

What This Framework Does: Compiles the Tilt binary, deploys servers to the
`tilt-integration` namespace, and cleans up when it finishes.

Each subdirectory is a test case driven by the file of the same name
(e.g., `oneup_.go` drives the test driven by the data in `oneup`).
Each Tiltfile should deploy small services to gcr.io/windmill-test-containers.
Add new images to purge-test-images.sh so that they get purged periodically.

Run the tests with

```
go test -tags 'integration' -timeout 300s ./integration
```

or

```
make integration
```

These tests will not run with the normal `make test`.

On CircleCI, we run these tests against a clean Kubernetes cluster.
Follow these instructions if you want to run the same cluster locally: https://github.com/kubernetes-sigs/kind
