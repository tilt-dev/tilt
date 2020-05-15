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
Follow these instructions if you want to run the same cluster locally.

https://github.com/kubernetes-sigs/kind

## Simulating `restart_container()` on non-Docker clusters
As of 6/28/19 `restart_container()`, a command that can be passed to a `live_update`, doesn't work on non-Docker clusters. However there's a workaround available to simulate `restart_container()`'s functionality. It's used in the onewatch integration test so that the test passes on non-Docker clusters. Here's how it works:

Copy start.sh and restart.sh to your container working dir.

Make your container entrypoint:
`./start.sh path-to-binary [args]`

To restart the container add this instead of `restart_container()` in the live update parameter:
`run(./restart.sh)`

So, for example:

```python
docker_build('gcr.io/windmill-test-containers/integration/onewatch',
             '.',
             dockerfile='Dockerfile',
             live_update=[
               sync('.', '/go/src/github.com/tilt-dev/tilt/integration/onewatch'),
               run('go install github.com/tilt-dev/tilt/integration/onewatch'),
               run('./restart.sh'),
             ])
```

This live update will cause the `go install` to be run in the container every time anything in the `.` path locally changes. After the `go install` is run, `./restart.sh` will be run. This will kill the original entrypoint, and restart it, effectively simulating the `container_restart()` functionality on Docker.
