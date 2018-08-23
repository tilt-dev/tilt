# Tilt

[![Build Status](https://circleci.com/gh/windmilleng/tilt/tree/master.svg?style=shield)](https://circleci.com/gh/windmilleng/tilt)
[![GoDoc](https://godoc.org/github.com/windmilleng/tilt?status.svg)](https://godoc.org/github.com/windmilleng/tilt)

## Installing
Run `go get -u github.com/windmilleng/tilt`

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
written in a Mill, a dialect of python. It's based on [starlark](https://github.com/bazelbuild/starlark), using the implementation in [go](https://github.com/google/skylark).

### Mill Builtins
Mill comes with built-in functions.

#### local_git_repo(path)
Creates a `repo` with the content at `path`.

* Args:
    * `path`: **str**
* Returns: **Repo**

#### build_docker_image(dockerfile_path, img_name, entrypoint)
Builds a docker image.

* Args:
  * `dockerfile_path`: **str**
  * `img_name`: **str**, e.g. blorgdev/backend or gcr.io/project-name/bucket-name
  * `entrypoint`: **str**
* Returns: **Image**

#### Image.add(path, repo)
Adds the content from `repo` into the image at `path`.

* Args:
  * `path`: **str**
  * `repo`: **Repo** (returned by `local_git_repo`)
* Returns: nothing

#### Image.run(cmd)
Runs `cmd` as a build step in the image.

* Args:
  * `cmd`: **str**
* Returns: nothing

#### k8s_service(yaml_text, img)
Creates a kubernetes service that tilt can deploy using the yaml text and the image passed in.

* Args:
  * `yaml_text`: **str** (text of yaml configuration)
  * `img`: **Image**
* Returns: **Service**

#### composite_service(services)
Creates a composite service; tilt will deploy (and watch) all services in `services`.

* Args:
  * `services`: array of **Service**
* Returns: **Service**

#### local(cmd)
Runs cmd, waits for it to finish, and returns its stdout.

* Args:
  * `cmd`: **str**
* Returns: **str**

## Developing
See DEVELOPING.md

## License
Copyright 2018 Windmill Engineering

Licensed under [the Apache License, Version 2.0](LICENSE)
