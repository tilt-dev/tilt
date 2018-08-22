# Tilt

[![Build Status](https://circleci.com/gh/windmilleng/tilt/tree/master.svg?style=shield)](https://circleci.com/gh/windmilleng/tilt)
[![GoDoc](https://godoc.org/github.com/windmilleng/tilt?status.svg)](https://godoc.org/github.com/windmilleng/tilt)


## Using tilt
`tilt up <service_name>` starts a service once; `tilt up --watch <service_name>` starts it and watches for changes.

Tilt reads from a Tiltfile. A simple Tiltfile is below:
```
def backend():
  repo = local_git_repo('../backend')
  img = build_docker_image('Dockerfile', 'companyname/backend', '/go/bin/server')
  img.add('/go/src/github.com/companyname/backend', repo)
  img.run('go install github.com/companyname/backend/server')
  return k8s_service(local_file('backend.yaml'), img)
```

## Mill
written in a Mill, a dialect of python.

### Mill Builtins
Mill comes with built-in functions.

### local_git_repo
`local_git_repo(path)` returns a `repo` with the content at `path`.

### build_docker_image
`build_docker_image(dockerfile_path, img_name, entrypoint)` builds a docker image.

### add
`img.add(path, repo)` adds the content from `repo` into the image at `path'.

### run
`img.run(cmd)` runs `cmd` as a build step in the image.

### k8s_service
`k8s_service(yaml_text, img)` declares a kubernetes service that tilt can deploy using the yaml text and the image passed in.

### composite_service
`composite_service([services])` creates a composite service; it will deploy (and watch) all `services`.

### local
`local(cmd)` runs cmd and returns its stdout.


## Prereqs
- `make`
- **[go 1.10](https://golang.org/dl/)**
- **errcheck**: `go get -u github.com/kisielk/errcheck`
- **protobuf 3.2**: `brew install protobuf` or install `protoc-3.2.0-[your_OS]` [from Github](https://github.com/google/protobuf/releases?after=v3.2.1)
- `wire` (`go get -u github.com/google/go-cloud/wire`)
- Our Python scripts are in Python 3.6.0. To run them:
  - **[pyenv](https://github.com/pyenv/pyenv#installation)**
  - **python**: `pyenv install`
  - if you're using GKE and get the error: "pyenv: python2: command not found", run:
    - `git clone git://github.com/concordusapps/pyenv-implict.git ~/.pyenv/plugins/pyenv-implict`

## Installing
Run `go get -u github.com/windmilleng/tilt`

## Developing
See `Makefile`.

## License
Copyright 2018 Windmill Engineering

Licensed under [the Apache License, Version 2.0](LICENSE)
