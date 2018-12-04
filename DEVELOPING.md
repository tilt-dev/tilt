## Prereqs
- `make`
- **[go 1.11](https://golang.org/dl/)**
- **errcheck**: `go get -u github.com/kisielk/errcheck`
- **protobuf 3.2**: `brew install protobuf` or install `protoc-3.2.0-[your_OS]` [from Github](https://github.com/google/protobuf/releases?after=v3.2.1)
- `wire` (`go get -u github.com/google/go-cloud/wire/cmd/wire`)
- Our Python scripts are in Python 3.6.0. To run them:
  - **[pyenv](https://github.com/pyenv/pyenv#installation)**
  - **python**: `pyenv install`
  - if you're using GKE and get the error: "pyenv: python2: command not found", run:
    - `git clone git://github.com/concordusapps/pyenv-implict.git ~/.pyenv/plugins/pyenv-implict`

## Developing
See `Makefile`.

## Documentation

The documentation is written in Restructured Text and generated with Sphinx. We install Sphinx inside
a container so that you don't have to install it locally. To regenerate the HTML, run

```
make docs
```

and open them [locally](docs/_build/html/index.html).

Netlify will automatically deploy the docs to [the public site](https://docs.windmill.build/) when you merge to master.

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
git tag -a v0.0.1 -m "my release"
git push origin v0.0.1
make release
```

goreleaser will build binaries for the latest tag (using semantic version to
determine "latest"). Check the current releases to figure out what the latest
release ought to be.

