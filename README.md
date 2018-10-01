# Tilt

[![Build Status](https://circleci.com/gh/windmilleng/tilt/tree/master.svg?style=shield)](https://circleci.com/gh/windmilleng/tilt)
[![GoDoc](https://godoc.org/github.com/windmilleng/tilt?status.svg)](https://godoc.org/github.com/windmilleng/tilt)

## Installing

Run `go get -u github.com/windmilleng/tilt`

## Using Tilt
`tilt up <service_name>` starts a service once; `tilt up --watch <service_name>` starts it and watches for changes.

Tilt reads from a Tiltfile. A simple Tiltfile is below:

```python
def backend():
  img = static_build('Dockerfile', 'gcr.io/companyname/backend')
  yaml = read_file('backend.yaml')
  return k8s_service(yaml, img)
```

## Optimizing Tilt

Building with the `Tiltfile` above may be slow because it builds a new image each time.

With a `Tiltfile` that uses `start_fast_build`, Tilt is able to optimize your build so
that it only runs the steps that have changed.

```python
def backend():
  start_fast_build('Dockerfile', 'gcr.io/companyname/backend', '/go/bin/server')
  repo = local_git_repo('.')
  add(repo, '/go/src/github.com/companyname/backend')
  run('cd /go/src/github.com/companyname/backend && npm install .',
      trigger=['package.json'])
  run('go install github.com/companyname/backend/server')
  img = stop_build()

  yaml = read_file('backend.yaml')
  return k8s_service(yaml, img)
```

## Mill

written in a Mill, a dialect of python. It's based on [starlark](https://github.com/bazelbuild/starlark), using the implementation in [go](https://github.com/google/skylark).

### Mill Builtins

Mill comes with built-in functions.

#### static_build(dockerfile, ref, context?)
Builds a docker image.

```python
def static_build(dockerfile: str, ref: str, context: str = "") -> Image:
      """Builds a docker image.

    Args:
      dockerfile: The path to a Dockerfile
      ref: e.g. a blorgdev/backend or gcr.io/project-name/bucket-name
      context?: The path to use as the Docker build context. Defaults to the Dockerfile directory.
    Returns:
      Image
    """
```

#### local_git_repo
Creates a `repo` from the git repo at `path`.

```python
def local_git_repo(path: str) -> Repo
```

#### Repo
Represents a local code repository

```python
class Repo:
  def path(path: str) -> localPath:
    """Returns the absolute path to the file specified at `path` in the repo.
    path must be a relative path.

    Args:
      path: relative path in repository
    Returns:
      A localPath resource, representing a local path on disk.
    """
```

#### start_fast_build

Initiates a docker image build that supports `add`s and `run`s, and that uses a cache for subsequent builds.

TODO(dmiller): explain how this is fast, and image vs container builds?
TODO(dmiller): explain the concept of the active build

```python
def start_fast_build(dockerfile_path: str, img_name: str, entrypoint: str = "") -> None
```

#### add

Adds the content from `src` into the image at path `dest`.

```python
def add(src: Union[localPath, Repo], dest: str) -> None
```

#### run

Runs `cmd` as a build step in the image.
If the `trigger` argument is specified, the build step is only run on changes to the given file(s).

```python
def run(cmd: str, trigger: Union[List[str], str] = []) -> None
```

#### Service

Represents a Kubernetes service that Tilt can deploy and monitor.

```python
class Service
```

#### k8s_service

Creates a kubernetes service that tilt can deploy using the yaml text and the image passed in.

```python
def k8s_service(yaml_text: str, img: Image) -> Service
```

#### Image

Represents a built Docker image

```python
class Image
```

#### composite_service

Creates a composite service; tilt will deploy (and watch) all services returned by the functions in `service_fns`.

```python
def composite_service(List[Callable[[], Service]]) -> Service
```

#### local

Runs cmd, waits for it to finish, and returns its stdout.

```python
def local(cmd: str) -> str
```

#### read_file

Reads file and returns its contents.

```python
def read_file(file_path: str) -> str
```

#### stop_build()

Closes the currently active build and returns a container Image that has all of the adds and runs applied.

```python
def stop_build() -> Image
```

## Developing
See [DEVELOPING.md](DEVELOPING.md)

## Privacy

This tool can send usage reports to https://events.windmill.build, to help us
understand what features people use. We only report on which `tilt` commands
run and how long they run for.

You can enable usage reports by running

```
tilt analytics opt in
```

(and disable them by running `tilt analytics opt out`.)

We do not report any personally identifiable information. We do not report any
identifiable data about your code.

We do not share this data with anyone who is not an employee of Windmill
Engineering. Data may be sent to third-party service providers like Datadog,
but only to help us analyze the data.

## License

Copyright 2018 Windmill Engineering

Licensed under [the Apache License, Version 2.0](LICENSE)
