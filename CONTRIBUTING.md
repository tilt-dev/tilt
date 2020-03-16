# Hacking on Tilt

So you want to make a change to `tilt`!

## Contributing

We welcome contributions, either as bug reports, feature requests, or pull requests.

We want everyone to feel at home in this repo and its environs; please see our [**Code of Conduct**](https://docs.tilt.dev/code_of_conduct.html) for some rules that govern everyone's participation.

Most of this page describes how to get set up making & testing changes. See a [YouTube walkthrough](https://youtu.be/oGC5O-BCBhc) showing some of the steps below, for macOS.

Small PRs are better than large ones. If you have an idea for a major feature, please file
an issue first. The [Roadmap](../../../../orgs/windmilleng/projects/3) has details on some of the upcoming
features that we have in mind and might already be in-progress.

## Build Prereqs

If you just want to build Tilt:

- **[make](https://www.gnu.org/software/make/)**
- **[go 1.14](https://golang.org/dl/)**
- **[golangci-lint](https://github.com/golangci/golangci-lint)** (to run lint)
- [yarn](https://yarnpkg.com/lang/en/docs/install/) (for JS resources)

## Test Prereqs

If you want to run the tests:

- **[docker](https://docs.docker.com/install/)** - Many of the `tilt` build steps do work inside of containers
  so that you don't need to install extra toolchains locally (e.g., the protobuf compiler).
- **[kubectl](https://kubernetes.io/docs/tasks/tools/install-kubectl/)**
- **[kustomize 2.0 or higher](https://github.com/kubernetes-sigs/kustomize)**: `go get -u sigs.k8s.io/kustomize`
- **[helm](https://docs.helm.sh/using_helm/#installing-helm)**
- **[docker compose](https://docs.docker.com/compose/install/)**: NOTE: this doesn't need to be installed separately from Docker on macOS
- **[jq](https://stedolan.github.io/jq/download/)**

## Optional Prereqs

Other development commands:

- **[goimports](https://godoc.org/golang.org/x/tools/cmd/goimports)**: `go get -u golang.org/x/tools/cmd/goimports` (to sort imports, IDE-specific installation instructions in the link). You should configure goimports to run with `-local github.com/windmill/tilt`
- **[toast](https://github.com/stepchowfun/toast)**: `curl https://raw.githubusercontent.com/stepchowfun/toast/master/install.sh -LSfs | sh` Used for generating some protobuf files
- Our Python scripts are in Python 3.6.0. To run them:
  - **[pyenv](https://github.com/pyenv/pyenv#installation)**
  - **python**: `pyenv install`
  - if you're using GKE and get the error: "pyenv: python2: command not found", run:
    - `git clone git://github.com/concordusapps/pyenv-implict.git ~/.pyenv/plugins/pyenv-implict`

## Developing

To check out Tilt for the first time, run:

```
go get -u github.com/windmilleng/tilt/cmd/tilt
```

The Go toolchain will checkout the Tilt repo somewhere on your GOPATH,
usually under `~/go/src/github.com/windmilleng/tilt`.

To run the fast test suite, run:

```
make shorttest
```

To run the slow test suite that interacts with Docker and builds real images, run:

```
make test
```

If you want to run an integration test suite that deploys servers to Kubernetes and
verifies them, run:

```
make integration
```

To install `tilt` on PATH, run

```
make install
```

To start using Tilt, just run `tilt up` in any project with a `Tiltfile` -- i.e., NOT the root of the Tilt source code.
There are plenty of toy projects to play with in the [integration](https://github.com/windmilleng/tilt/tree/master/integration) directory
(see e.g. `./integration/oneup`), or check out one of these sample repos to get started:
- [ABC123](https://github.com/windmilleng/abc123): Go/Python/JavaScript microservices generating random letters and numbers
- [Servantes](https://github.com/windmilleng/servantes): a-little-bit-of-everything sample app with multiple microservices in different languages, showcasing many different Tilt behaviors
- [Frontend Demo](https://github.com/windmilleng/tilt-frontend-demo): Tilt + ReactJS
- [Live Update Examples](https://github.com/windmilleng/live_update): contains Go and Python examples of Tilt's [Live Update](https://docs.tilt.dev/live_update_tutorial.html) functionality
- [Sidecar Example](https://github.com/windmilleng/sidecar_example): simple Python app and home-rolled logging sidecar

## Performance

### Go Profile

Tilt exposes the standard Go pprof hooks over [HTTP](https://golang.org/pkg/net/http/pprof/).

To look at a 30-second CPU profile:

```
go tool pprof http://localhost:10350/debug/pprof/profile?seconds=30
```

To look at the heap profile:

```
go tool pprof http://localhost:10350/debug/pprof/heap
```

This opens a special REPL that lets you explore the data.
Type `web` in the REPL to see a CPU graph.

For more information on pprof, see https://github.com/google/pprof/blob/master/doc/README.md.

### Opentracing
If you're trying to diagnose Tilt performance problems that lie between Tilt and your Kubernetes cluster (or between Tilt and Docker) traces can be helpful. The easiest way to get started with Tilt's [opentracing](https://opentracing.io/) support is to use the [Jaeger all-in-one image](https://www.jaegertracing.io/docs/1.11/getting-started/#all-in-one).

```
$ docker run -d --name jaeger \
  -e COLLECTOR_ZIPKIN_HTTP_PORT=9411 \
  -p 5775:5775/udp \
  -p 6831:6831/udp \
  -p 6832:6832/udp \
  -p 5778:5778 \
  -p 16686:16686 \
  -p 14268:14268 \
  -p 9411:9411 \
  jaegertracing/all-in-one:1.11
```

Then start Tilt with the following flags:

```
tilt up --trace --traceBackend jaeger
```

When Tilt starts one of the first lines in the log output should contain a trace ID, like so:

```
TraceID: 26256f1f6aa875e5
```

You can use the Jaeger UI (by default running on http://localhost:16686/) to query for this span and see all of the traces for the current Tilt run. These traces are made available immediately as you use Tilt. You don't need to wait until after Tilt has stopped to get access to the tracing data.

## Web UI

`tilt` uses a web interface for logs investigation.

By default, the web interface runs on port 10350.

When you use a released version of Tilt, all the HTML, CSS, and JS assets are served from our
[production bucket](https://console.cloud.google.com/storage/browser/tilt-static-assets).

When you build Tilt from head, the Tilt binary will default to development mode.
When you run Tilt, it will run a webpack dev server as a separate process on port 46764,
and reverse proxy all asset requests to the dev server.

To manually control the assets served, you can use:

```
tilt up --web-mode=local
```

to force Tilt to use the webpack dev server, or you can use

```
tilt up --web-mode=prod
```

to force Tilt to use production assets.


To run the server on an alternate port (e.g. 8001):

```
tilt up --port=8001
```

## Documentation

The landing page and documentation lives in
[the tilt.build repo](https://github.com/windmilleng/tilt.build/).

We write our docs in Markdown and generate static HTML with [Jekyll](https://jekyllrb.com/).

Netlify will automatically deploy the docs to [the public site](https://docs.tilt.dev/)
when you merge to master.

## Wire

Tilt uses [wire](https://github.com/google/wire) for dependency injection. It
generates all the code in the wire_gen.go files.

`make wire-dev` runs `wire` locally and ensures you have fast feedback when
rebuilding the generated code.

`make wire` runs `wire` in a container, to ensure you're using the correct
version.

What do you do if you added a dependency, and `make wire` is failing?

### A Practical Guide to Fixing Your Dependency Injector

(This guide will work with any Dependency Injector - Dagger, Guice, etc - but is
written for Wire)

Step 1) DON'T PANIC. Fixing a dependency injector is like untangling a hair
knot. If you start pushing and pulling dependencies in the middle of the graph,
you will make it much worse.

Step 2) Run `make wire-dev`

Step 3) Look closely at the error message. Identify the "top" of the dependency
graph that is failing. So if your error message is:

```
wire: /go/src/github.com/windmilleng/tilt/internal/cli/wire.go:182:1: inject wireRuntime: no provider found for github.com/windmilleng/tilt/internal/k8s.MinikubeClient
	needed by github.com/windmilleng/tilt/internal/k8s.Client in provider set "K8sWireSet" (/go/src/github.com/windmilleng/tilt/internal/cli/wire.go:44:18)
	needed by github.com/windmilleng/tilt/internal/container.Runtime in provider set "K8sWireSet" (/go/src/github.com/windmilleng/tilt/internal/cli/wire.go:44:18)
wire: github.com/windmilleng/tilt/internal/cli: generate failed
wire: at least one generate failure
```

then the "top" is the function wireRuntime at wire.go:182.

Step 4) Identify the dependency that is missing. In the above example, that
dependency is MinikubeClient.

Step 5) At the top-level provider function, add a provider for the missing
dependency. In this example, that means we add ProvideMinikubeClient to the
wire.Build call in wireRuntime.

Step 6) Go back to Step (2), and repeat until all errors are gone

Final Note: All dependency injection systems have a notion of groups of common
dependencies (in Wire, they're called WireSets). When fixing an injection error,
you generally want to move providers "up" the graph. i.e., remove them from
WireSets and add them to wire.Build calls. It's OK if this leads to lots of
duplication. Later, you can refactor them back down into common WireSets once
you've got it working.

## Releasing

We use [goreleaser](https://goreleaser.com) for releases.

Requirements:
- goreleaser: `go get -u github.com/goreleaser/goreleaser`
- MacOS
- Python
- [gsutil](https://cloud.google.com/storage/docs/gsutil_install)
- `GITHUB_TOKEN` env variable with repo scope

Currently, releases have to be built on MacOS due to cross-compilation issues with Apple FSEvents.
Cross-compiling a Linux target binary with a MacOS toolchain works fine.

To create a new release at tag `$TAG`:

```
git fetch --tags
git tag -a v0.0.1 -m "my release"
git push origin v0.0.1
make release
```

goreleaser will build binaries for the latest tag (using semantic version to
determine "latest"). Check the current releases to figure out what the latest
release ought to be.

After updating the release notes,
update the [install](https://github.com/windmilleng/tilt.build/tree/master/docs/install.md) and [upgrade](https://github.com/windmilleng/tilt.build/blob/master/docs/upgrade.md) docs,
the [default dev version](internal/cli/build.go),
and the [installer version](scripts/install.sh).

To auto-generate new CLI docs, make sure you have tilt.build in a sibling directory of tilt, and run:

```
make cli-docs
```

### Version numbers
For pre-v1.0:
* If adding backwards-compatible functionality increment the patch version (0.x.Y).
* If adding backwards-incompatible functionality increment the minor version (0.X.y). We would probably **write a blog post** about this.

### Releasing the Synclet

Releasing a synclet should be very infrequent, because the amount of things it
does is small. (It's basically an optimization over `kubectl cp`, `kubectl
exec`, and restarting a container.)

To release a synclet, run `make synclet-release`. This will automatically:

- Publish a new synclet image tagged with the current date
- Update [sidecar.go](internal/synclet/sidecar/sidecar.go) with the new tag

Then submit the PR. The next time someone releases Tilt, it will use the new image tag.



