# Hacking on Tilt

So you want to make a change to `tilt`!

## Contributing

We welcome contributions, either as bug reports, feature requests, or pull requests.

Most of this page describes how to get set up making & testing changes.

Small PRs are better than large ones. If you have an idea for a major feature, please file
an issue first. The [Roadmap](ROADMAP.md) has details on some of the upcoming
features that we have in mind and might already be in-progress.

## Necessary Prereqs
- **[make](https://www.gnu.org/software/make/)**
- **[go 1.11](https://golang.org/dl/)**
- **[errcheck](https://github.com/kisielk/errcheck)**: `go get -u github.com/kisielk/errcheck` (to run lint)
- **[docker](https://docs.docker.com/install/)** - Many of the `tilt` build steps do work inside of containers
  so that you don't need to install extra toolchains locally (e.g., the protobuf compiler).
- **[kubectl](https://kubernetes.io/docs/tasks/tools/install-kubectl/)** (for tests)
- **[kustomize](https://github.com/kubernetes-sigs/kustomize)**: `go get -u sigs.k8s.io/kustomize` (for tests)
- **[docker compose](https://docs.docker.com/compose/install/)**: (for tests) NOTE: this doesn't need to be installed separately from Docker on macOS

## Optional Prereqs
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

## Documentation

The documentation is written in Restructured Text and generated with Sphinx. We install Sphinx inside
a container so that you don't have to install it locally. To regenerate the HTML, run

```
make docs
```

and open them [locally](docs/_build/html/index.html).

Netlify will automatically deploy the docs to [the public site](https://docs.tilt.build/) when you merge to master.

If you'd like to send a preview to someone else,
push with the special branch name `docs` and open a pull request.
Netlify will annotate the pull request with the URL of a preview.

## Releasing

We use [goreleaser](https://goreleaser.com) for releases.

Requirements:
- goreleaser
- MacOS
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

