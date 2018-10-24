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

## Releasing

We use [goreleaser](https://goreleaser.com) for releases.

Requirements:
- goreleaser
- Linux
- `GITHUB_TOKEN` env variable with repo scope

Currently releases only work on Linux -> Linux due to cross-compilation issues with Apple FSEvents.

To create a new release at tag `$TAG`:

```
git tag -a v0.0.1 -m "First release"
git push origin v0.0.1
make release
```

goreleaser will build binaries for the latest tag (using semantic version to determine "latest").

