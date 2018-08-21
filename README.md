# Tilt

[![Build Status](https://circleci.com/gh/windmilleng/tilt/tree/master.svg?style=shield)](https://circleci.com/gh/windmilleng/tilt)
[![GoDoc](https://godoc.org/github.com/windmilleng/tilt?status.svg)](https://godoc.org/github.com/windmilleng/tilt)

## Prereqs
- `make`
- **[go 1.10](https://golang.org/dl/)**
- **protobuf 3.2**: `brew install protobuf` or install `protoc-3.2.0-[your_OS]` [from Github](https://github.com/google/protobuf/releases?after=v3.2.1)
- `wire` (`go get -u github.com/google/go-cloud/wire`)
- Our Python scripts are in Python 3.6.0. To run them:
  - **[pyenv](https://github.com/pyenv/pyenv#installation)**
  - **python**: `pyenv install`
  - if you're using GKE and get the error: "pyenv: python2: command not found", run:
    - `git clone git://github.com/concordusapps/pyenv-implict.git ~/.pyenv/plugins/pyenv-implict`

## Developing
Run `make` from the root of the repo to generate all protobuf files.

## License
Copyright 2018 Windmill Engineering

Licensed under [the Apache License, Version 2.0](LICENSE)
