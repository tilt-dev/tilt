# Hacking on Tilt

So you want to make a change to `tilt`!

## Contributing

We welcome contributions, either as bug reports, feature requests, or pull requests.

We want everyone to feel at home in this repo and its environs; please see our [**Code of Conduct**](https://docs.tilt.dev/code_of_conduct.html) for some rules that govern everyone's participation.

Most of this page describes how to get set up making & testing changes.

Small PRs are better than large ones. If you have an idea for a major feature, please file
an issue first. The [Roadmap](ROADMAP.md) has details on some of the upcoming
features that we have in mind and might already be in-progress.

## Build Prereqs

If you just want to build Tilt:

- **[make](https://www.gnu.org/software/make/)**
- **[go 1.12](https://golang.org/dl/)**
- **[errcheck](https://github.com/kisielk/errcheck)**: `go get -u github.com/kisielk/errcheck` (to run lint)

## Test Prereqs

If you want to run the tests:

- **[docker](https://docs.docker.com/install/)** - Many of the `tilt` build steps do work inside of containers
  so that you don't need to install extra toolchains locally (e.g., the protobuf compiler).
- **[kubectl](https://kubernetes.io/docs/tasks/tools/install-kubectl/)**
- **[kustomize](https://github.com/kubernetes-sigs/kustomize)**: `go get -u sigs.k8s.io/kustomize`
- **[helm](https://docs.helm.sh/using_helm/#installing-helm)**
- **[docker compose](https://docs.docker.com/compose/install/)**: NOTE: this doesn't need to be installed separately from Docker on macOS

## Optional Prereqs

Other development commands:

- **[wire](https://github.com/google/wire)**: `go get -u github.com/google/wire/cmd/wire` (to update generated dependency injection code)
- Our Python scripts are in Python 3.6.0. To run them:
  - **[pyenv](https://github.com/pyenv/pyenv#installation)**
  - **python**: `pyenv install`
  - if you're using GKE and get the error: "pyenv: python2: command not found", run:
    - `git clone git://github.com/concordusapps/pyenv-implict.git ~/.pyenv/plugins/pyenv-implict`

## Developing

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

## Web UI

`tilt` has an experimental web interface! We currently use it to run experiments.

By default, the web interface is off. To run it on port 8001:

```
tilt up --port=8001
```

This will server HTML, CSS, and JS from our
[production bucket](https://console.cloud.google.com/storage/browser/tilt-static-assets).

To develop on the interface locally, run:


```
tilt up --port=8001 --web-mode=local
```

This will run a webpack dev server as a separate process,
and reverse proxy all asset requests to the dev server.

## Documentation

The landing page and documentation lives in
[the tilt.build repo](https://github.com/windmilleng/tilt.build/).

We write our docs in Markdown and generate static HTML with [Jekyll](https://jekyllrb.com/).

Netlify will automatically deploy the docs to [the public site](https://docs.tilt.dev/)
when you merge to master.

## Releasing

We use [goreleaser](https://goreleaser.com) for releases.

Requirements:
- goreleaser: `go get -u github.com/goreleaser/goreleaser`
- MacOS
- Python
- [gsutil](https://cloud.google.com/storage/docs/gsutil_install)
- [yarn](https://yarnpkg.com/lang/en/docs/install/)
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

After updating the release notes, update [the docs](https://github.com/windmilleng/tilt.build/tree/master/docs/install.md)
and [the default dev version](internal/cli/build.go).

### Version numbers
Our rule of thumb pre 1.0 is only bump the minor version if you would write a blog post about it. (We haven't always followed this rule, but we'd like to start!)
