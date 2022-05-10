# Hacking on Tilt

So you want to make a change to `tilt`!

## Guidelines

We welcome contributions, either as bug reports, feature requests, or pull requests.

We want everyone to feel at home in this repo and community!
Please read our [**Code of Conduct**](https://docs.tilt.dev/code_of_conduct.html) for some rules that govern everyone's participation.

Most of this page describes how to get set up making & testing changes.
See a [YouTube walkthrough](https://youtu.be/oGC5O-BCBhc) showing some of the steps below, for macOS.

Small PRs are better than large ones.
If you have an idea for a major feature, please file an issue first.

## Clone
To check out Tilt for the first time, run:

```
git clone https://github.com/tilt-dev/tilt.git
```

## Build
### Prerequisites
If you just want to build Tilt:

- **[make](https://www.gnu.org/software/make/)**
- **[go](https://golang.org/dl/)** (see `go.mod` for supported version)
- **C/C++ toolchain** (for CGO dependencies)
- **[golangci-lint](https://github.com/golangci/golangci-lint)** (to run lint) 

To use the local Webpack server for UI (default for locally compiled versions of Tilt):
- **[Node.js](https://nodejs.org/en/download/)** (LTS - see `.engines.node` in `web/package.json`)
- **[yarn](https://yarnpkg.com/lang/en/docs/install/)**

### Build & Install From Source
To install `tilt` on PATH, run:

```
make build-js
make install
```

> Running the `build-js` task is currently optional but _highly_ recommended.
> If available, the build will embed the frontend assets in the `tilt` binary,
> which allows Tilt to work offline. Otherwise, assets will be served at runtime
> from a remote server.

This will install the new `tilt` binary in `$GOPATH/bin` - typically `$HOME/go/bin`.
You can verify this is the binary you just built with:
```
"$(go env GOPATH)/bin/tilt" version
```

The build date should match the current date.
Be aware that you might already have a `tilt` binary in your $PATH, so running `tilt` without specifying exactly which `tilt` binary you want might have you running the wrong binary.

### Running
To start using Tilt, run `tilt up` in any project with a `Tiltfile` -- i.e., NOT the root of the Tilt source code.
There are plenty of toy projects to play with in the [integration](https://github.com/tilt-dev/tilt/tree/master/integration) directory
(see e.g. `./integration/oneup`), or check out one of these sample repos to get started:
- [ABC123](https://github.com/tilt-dev/abc123): Go/Python/JavaScript microservices generating random letters and numbers
- [Servantes](https://github.com/tilt-dev/servantes): a-little-bit-of-everything sample app with multiple microservices in different languages, showcasing many different Tilt behaviors
- [Frontend Demo](https://github.com/tilt-dev/tilt-frontend-demo): Tilt + ReactJS
- [Live Update Examples](https://github.com/tilt-dev/live_update): contains Go and Python examples of Tilt's [Live Update](https://docs.tilt.dev/live_update_tutorial.html) functionality
- [Sidecar Example](https://github.com/tilt-dev/sidecar_example): simple Python app and home-rolled logging sidecar

## Test
### Prerequisites
If you want to run the tests:

- **[docker](https://docs.docker.com/install/)** - Many of the `tilt` build steps do work inside of containers
  so that you don't need to install extra toolchains locally (e.g., the protobuf compiler).
- **[kubectl](https://kubernetes.io/docs/tasks/tools/install-kubectl/)**
- **[kustomize 2.0 or higher](https://github.com/kubernetes-sigs/kustomize)**: `go get -u sigs.k8s.io/kustomize`
- **[helm](https://docs.helm.sh/using_helm/#installing-helm)**
- **[docker compose](https://docs.docker.com/compose/install/)**: NOTE: this doesn't need to be installed separately from Docker on macOS
- **[jq](https://stedolan.github.io/jq/download/)**

### Running Test Suite (Fast)
To run the fast test suite, run:

```
make shorttest
```

### Running Test Suite (Slow)
To run the slow test suite that interacts with Docker and builds real images, run:

```
make test
```

### Running Integration Tests
If you want to run an integration test suite that deploys servers to Kubernetes and
verifies them, run:

```
make integration
```

### Optional/Other

Other development commands:

- **[goimports](https://pkg.go.dev/golang.org/x/tools/cmd/goimports?tab=doc)**: `go install golang.org/x/tools/cmd/goimports@latest` (to sort imports)
  - Run manually with `make goimports`
  - Run automatically with IDE
    - See goimports docs for IDE specific configuration instructions
    - Run with `-local github.com/tilt-dev`
- **[toast](https://github.com/stepchowfun/toast)**: `curl https://raw.githubusercontent.com/stepchowfun/toast/master/install.sh -LSfs | sh` (local development tasks)

## Tilt APIServer
The Tilt APIServer is our new system for managing Tilt internals:
https://github.com/tilt-dev/tilt-apiserver

To add a new first-party type, run:

```
scripts/api-new-type.sh MyResourceType
```

and follow the instructions.

Once you've added fields for your type, run:

```
scripts/update-codegen.sh
```

to regenerate client code for reading and writing the new type.

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

## Web UI

`tilt up` runs a web server hosting a React single page application on port 10350 (customizable with `--port` or `TILT_PORT`).

### Web Mode `(--web-mode)`
There are several possibilities for how Tilt serves the web assets based on the
build configuration.

#### Local (Dev)
By default, non-release builds of Tilt use a local Webpack dev server.
When Tilt first starts, it will launch the Webpack dev server for you.
If you immediately open the Tilt web UI, you might get an error message until Webpack has finished starting.
The page should auto-reload once Webpack is ready.

To force Tilt to use the Webpack dev server, launch with `tilt up --web-mode=local`.

#### Embedded
If bundled JS assets are available while building Tilt, they will be included in the binary and served via embedded mode.
This ensures the local Tilt server is self-contained and does not require internet access for the web UI.

This is the default for Tilt releases starting with v0.27.0.

To force Tilt to use the embedded assets, launch with `tilt up --web-mode=embedded`.

If unavailable, Tilt will refuse to start with an error:
```
Error: requested embedded mode, but assets are not available
```
To fix this, run `make build-js` and then re-build Tilt (e.g. with `make install`).

#### Cloud (Deprecated)
In the remote/production mode, all the HTML, CSS, and JS assets are served from our
[production bucket](https://console.cloud.google.com/storage/browser/tilt-static-assets).

This was the default for Tilt releases until v0.27.0.

To force Tilt to use the remote production assets, launch with `tilt up --web-mode=cloud`.
During development, this can speed up startup if you are not making changes to the frontend and does not require a local NodeJS toolchain.

### Local Snapshot Mode
You can view a locally running Tilt session as though it was a snapshot by tweaking the URL to be `/snapshot/snapshot_id/overview`.
(The `snapshot_id` portion of the URL can be any valid identifier.)
For example, http://localhost:10350/snapshot/aaaa/overview.

Please note this uses a serialized version of the webview/snapshot generated by the Tilt server, so it might behave slightly differently than a real snapshot.

### Lint (`prettier` + `eslint`)
To format all files with Prettier, run `make prettier` from the repo root or `yarn prettier` from `web/`.

To run lint checks with ESLint (and auto-fix any trivial issues), run `yarn eslint`.

To **verify** that there are no formatting/lint violations, but _not_ auto-fix, run `make check-js` from the repo root or `yarn check` from `web/`.

### Tests
To run all tests, you can run `make test-js` from the repo root.

If you are actively developing, running `yarn test` from `web/` will launch Jest in interactive mode,
which can auto re-run affected tests and more.

#### Updating Jest Snapshot Tests
First, double check that the element render has changed _by design_ and not as a result of a regression.

The interactive mode of Jest will guide you to update snapshots.
See the [Jest snapshot testing documentation](https://jestjs.io/docs/en/snapshot-testing#interactive-snapshot-mode) for details.

## Documentation

The user-facing landing page and documentation lives in
[the tilt.build repo](https://github.com/tilt-dev/tilt.build/).

We write our docs in Markdown and generate static HTML with [Jekyll](https://jekyllrb.com/).

Netlify will automatically deploy the docs to [the public site](https://docs.tilt.dev/)
when you merge to master.

For internal architecture, see [the Tilt Architecture Guide](internal/README.md).

## Troubleshooting
### Force Sign Out of Tilt Cloud

Once you've connected Tilt to Tilt Cloud via GitHub, you cannot sign out to break the connection.
But sometimes during development and testing, you need to do this. Remove the token file named `token`
located at `~/.windmill` on your machine. Restart Tilt, and you will be signed out.

### Dependency Injection (`wire`)

Tilt uses [wire](https://github.com/google/wire) for dependency injection. It
generates all the code in the wire_gen.go files.

`make wire-dev` runs `wire` locally and ensures you have fast feedback when
rebuilding the generated code.

`make wire` runs `wire` in a container, to ensure you're using the correct
version.

What do you do if you added a dependency, and `make wire` is failing?

#### A Practical Guide to Fixing Your Dependency Injector

(This guide will work with any Dependency Injector - Dagger, Guice, etc - but is
written for Wire)

Step 1) DON'T PANIC. Fixing a dependency injector is like untangling a hair
knot. If you start pushing and pulling dependencies in the middle of the graph,
you will make it much worse.

Step 2) Run `make wire-dev`

Step 3) Look closely at the error message. Identify the "top" of the dependency
graph that is failing. So if your error message is:

```
wire: /go/src/github.com/tilt-dev/tilt/internal/cli/wire.go:182:1: inject wireRuntime: no provider found for github.com/tilt-dev/tilt/internal/k8s.MinikubeClient
	needed by github.com/tilt-dev/tilt/internal/k8s.Client in provider set "K8sWireSet" (/go/src/github.com/tilt-dev/tilt/internal/cli/wire.go:44:18)
	needed by github.com/tilt-dev/tilt/internal/container.Runtime in provider set "K8sWireSet" (/go/src/github.com/tilt-dev/tilt/internal/cli/wire.go:44:18)
wire: github.com/tilt-dev/tilt/internal/cli: generate failed
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

We use [goreleaser](https://goreleaser.com) to publish binaries. We never run it
locally. We run it in a CircleCI container.

To create a new release at tag `$TAG`, in the `~/go/src/github.com/tilt-dev/tilt`
directory, first switch to `master` and pull the latest changes with `git pull`.
And then:

```
git fetch --tags
git tag -a v0.x.y -m "v0.x.y"
git push origin v0.x.y
```

CircleCI will automatically start building your release, and notify the
#notify-circleci slack channel when it's done. The releaser generates a release on
at https://github.com/tilt-dev/tilt/releases, with a Changelog prepopulated automatically.
(Give it a few moments. It appears as a tag first, before turning into a full release.)

### Version numbers
For pre-v1.0:
* If adding backwards-compatible functionality increment the patch version (0.x.Y).
* If adding backwards-incompatible functionality increment the minor version (0.X.y). We would probably **write a blog post** about this.
