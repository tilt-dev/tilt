# Tilt

[![Build Status](https://circleci.com/gh/windmilleng/tilt/tree/master.svg?style=shield)](https://circleci.com/gh/windmilleng/tilt)
[![GoDoc](https://godoc.org/github.com/windmilleng/tilt?status.svg)](https://godoc.org/github.com/windmilleng/tilt)

## Installing
Run `go get -u github.com/windmilleng/tilt`

## Using tilt
`tilt up <service_name>` starts a service once; `tilt up --watch <service_name>` starts it and watches for changes.

Tilt reads from a Tiltfile. A simple Tiltfile is below:
```python
def backend():
  start_fast_build('Dockerfile', 'companyname/backend', '/go/bin/server')
  repo = local_git_repo('.')
  add(repo, '/go/src/github.com/companyname/backend')
  run('go install github.com/companyname/backend/server')
  img = stop_build()

  return k8s_service(read_file('backend.yaml'), img)
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

#### Repo.path(path)
Gets the absolute to the file specified at `path` in the repo. Must be a relative path.

* Args:
  * `path`: **str**
* Returns:  **localPath**

#### start_fast_build(dockerfile_path, img_name, entrypoint?)
Builds a docker image.

* Args:
  * `dockerfile_path`: **str**
  * `img_name`: **str**, e.g. blorgdev/backend or gcr.io/project-name/bucket-name
  * `entrypoint?`: **str**
* Returns: **Image**

#### add(src, dest)
Adds the content from `src` into the image at path `dest`. Paths must be relative.

* Args:
  * `src`: **localPath|gitRepo**
  * `dest`: **str**
* Returns: nothing

#### run(cmd, trigger?)
Runs `cmd` as a build step in the image.
If the `trigger` file is specified, the build step is only run if the file is changed. Path must be relative.

* Args:
  * `cmd`: **str**
  * `trigger?`: **List[str] | str**
* Returns: nothing

#### k8s_service(yaml_text, img)
Creates a kubernetes service that tilt can deploy using the yaml text and the image passed in.

* Args:
  * `yaml_text`: **str** (text of yaml configuration)
  * `img`: **Image**
* Returns: **Service**

#### composite_service(service_fns)
Creates a composite service; tilt will deploy (and watch) all services returned by the functions in `service_fns`.

* Args:
  * `service_fns`: array of functions that each return **Service**
* Returns: **Service**

#### local(cmd)
Runs cmd, waits for it to finish, and returns its stdout.

* Args:
  * `cmd`: **str**
* Returns: **str**

#### read_file(file_path)
Reads file and returns its contents.

* Args:
  * `file_path`: **str**
* Returns: **str**

#### stop_build(file_path)
Closes the currently active build and returns a container Image that has all of the adds and runs applied.

* Returns: **Image**

## Developing
See DEVELOPING.md


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
Engineering.  Data may be sent to third-party service providers like Datadog,
but only to help us analyze the data.

## License
Copyright 2018 Windmill Engineering

Licensed under [the Apache License, Version 2.0](LICENSE)
