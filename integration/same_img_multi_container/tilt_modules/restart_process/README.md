# Restart Process

This extension helps create images that can restart on `live_update`:

- `docker_build_with_restart`: wraps a `docker_build` call
- `custom_build_with_restart`: wraps a `custom_build` call

At the end of a `live_update`, the container's process will rerun itself.

(Use it in place of the `restart_container()` Live Update step, which has been deprecated for Kubernetes resources.)

## When to Use
Use this extension when you have an image and you want to re-execute its entrypoint/command as part of a `live_update`.

E.g. if your app is a static binary, you'll probably need to re-execute the binary for any changes you made to take effect.

(If your app has hot reloading capabilities--i.e. it can detect and incorporate changes to its source code without needing to restart--you probably don't need this extension.)

### Unsupported Cases
This extension does NOT support process restarts for:
- Images built with `custom_build` using any of the `skips_local_docker`, `disable_push`, or `tag` parameters.
- Images run in Docker Compose resources (use the [`restart_container()`](https://docs.tilt.dev/api.html#api.restart_container) builtin instead)
- Images without a shell (e.g. `scratch`, `distroless`)
- Container commands specified as `command` in Kubernetes YAML will be overridden by this extension.
  - However, the `args` field is still available; [reach out](https://tilt.dev/contact) if you need help navigating the interplay between Tilt and these YAML values
- CRDs

If this extension doesn't work for your use case, [see our docs for alternatives](https://docs.tilt.dev/live_update_reference.html#restarting-your-process).

Run into a bug? Need a use case that we don't yet support? Let us know--[open an issue](https://github.com/tilt-dev/tilt-extensions/issues) or [contact us](https://tilt.dev/contact).

## How to Use

Import this extension by putting the following at the top of your Tiltfile:
```python
load('ext://restart_process', 'docker_build_with_restart')
```

For the image that needs the process restart, replace your existing `docker_build` call:
```python
docker_build(
    'foo-image',
    './foo',
    arg1=val1,
    arg2=val2,
    live_update=[x, y, z...]
)
```
with a `docker_build_with_restart` call:
```python
docker_build_with_restart(
    'foo-image',
    './foo',
    entrypoint='/go/bin/foo',
    arg1=val1,
    arg2=val2,
    live_update=[x, y, z...]
)
```
The call above looks just like the initial `docker_build` call except for one added parameter, `entrypoint` (in this example, `/go/bin/foo`). This is the command that you want to run on container start and _re-run_ on Live Update.

A custom_build call looks similar:

```python
load('ext://restart_process', 'custom_build_with_restart')

custom_build_with_restart(
    'foo-image',
    'docker build -t $EXPECTED_REF ./foo',
    deps=['./foo'],
    live_update=[sync(...)]
)
```

### Troubleshooting
#### `failed running [touch /tmp/.restart-proc']`
If you see an error of the form:
```
ERROR: Build Failed: ImageBuild: executor failed running [touch /tmp/.restart-proc']: exit code: 1
```
this often means that your Dockerfile user ([see docs](https://docs.docker.com/engine/reference/builder/#user)) doesn't have permission to write to the file we use to signal a process restart. Use the `restart_file` parameter to specify a file that your Dockerfile user definitely has write access to.

### API
```python
def docker_build_with_restart(ref: str, context: str,
    entrypoint: Union[str, List[str]],
    live_update: List[LiveUpdateStep],
    base_suffix: str = '-base',
    restart_file: str = '/.restart-proc',
    trigger: Union[str, List[str]] = [],
    **kwargs
):
    """Args:
      ref: name for this image (e.g. 'myproj/backend' or 'myregistry/myproj/backend'); as the parameter of the same name in docker_build
      context: path to use as the Docker build context; as the parameter of the same name in docker_build
      entrypoint: the command to be (re-)executed when the container starts or when a live_update is run
      live_update: set of steps for updating a running container; as the parameter of the same name in docker_build
      base_suffix: suffix for naming the base image, applied as {ref}{base_suffix}
      restart_file: file that Tilt will update during a live_update to signal the entrypoint to rerun
      trigger: (optional) list of local paths. If specified, the process will ONLY be restarted when there are changes
               to the given file(s); as the parameter of the same name in the LiveUpdate `run` step.
      **kwargs: will be passed to the underlying `docker_build` call
    """
    

def custom_build_with_restart(ref: str, command: str, deps: List[str], entrypoint,

    entrypoint: Union[str, List[str]],
    live_update: List[LiveUpdateStep],
    base_suffix: str = '-base',
    restart_file: str = '/.restart-proc',
    trigger: Union[str, List[str]] = [],
    , **kwargs
):
    """
     Args:
      ref: name for this image (e.g. 'myproj/backend' or 'myregistry/myproj/backend'); as the parameter of the same name in custom_build
      command: build command for building your image
      deps: source dependencies of the custom build
      entrypoint: the command to be (re-)executed when the container starts or when a live_update is run
      live_update: set of steps for updating a running container; as the parameter of the same name in custom_build
      base_suffix: suffix for naming the base image, applied as {ref}{base_suffix}
      restart_file: file that Tilt will update during a live_update to signal the entrypoint to rerun
      trigger: (optional) list of local paths. If specified, the process will ONLY be restarted when there are changes
               to the given file(s); as the parameter of the same name in the LiveUpdate `run` step.
      **kwargs: will be passed to the underlying `custom_build` call
    """
```

## What's Happening Under the Hood
*If you're a casual user/just want to get your app running, you can stop reading now. However, if you want to dig deep and know exactly what's going on, or are trying to debug weird behavior, read on.*

This extension wraps commands in `tilt-restart-wrapper`, which makes use of [`entr`](https://github.com/eradman/entr/)
to run arbitrary commands whenever a specified file changes. Specifically, we override the container's entrypoint with the following:

```
/tilt-restart-wrapper --watch_file='/.restart-proc' <entrypoint>
```

This invocation says:
- when the container starts, run <entrypoint>
- whenever the `/.restart-proc` file changes, re-execute <entrypoint>

We also set the following as the last `live_update` step:
```python
run('date > /.restart-proc')
```

Because `tilt-restart-wrapper` will re-execute the entrypoint whenever `/.restart-proc'` changes, the above `run` step will cause the entrypoint to re-run.

#### Provide `tilt-restart-wrapper`
For this all to work, the `entr` binary must be available on the Docker image. The easiest solution would be to call e.g. `apt-get install entr` in the Dockerfile, but different base images will have different package managers; rather than grapple with that, we've made a statically linked binary available on Docker image: [`tiltdev/entr`](https://hub.docker.com/repository/docker/tiltdev/entr).

To build `image-foo`, this extension will:
- build your image as normal (via `docker_build`, with all of your specified args/kwargs) but with the name `image-foo-base`
- build `image-foo` (the actual image that will be used in your resource) as a _child_ of `image-foo-base`, with the `tilt-process-wrapper` and its dependencies available

Thus, the final image produced is tagged `image-foo` and has all the properties of your original `docker_build`, plus access to the `tilt-restart-wrapper` binary.

#### Why a Wrapper?
Why bother with `tilt-restart-wrapper` rather than just calling `entr` directly?

Because in its canonical invocation, `entr` requires that the file(s) to watch be piped via stdin, i.e. it is invoked like:
```
echo "/.restart-proc" | entr -rz /bin/my-app
```

When specified as a `command` in Kubernetes or Docker Compose YAML (this is how Tilt overrides entrypoints), the above would therefore need to be executed as shell:
```
/bin/sh -c 'echo "/.restart-proc" | entr -rz /bin/my-app'
```
Any `args` specified in Kubernetes/Docker Compose are attached to the end of this call, and therefore in this case would apply TO THE `/bin/sh -c` CALL, rather than to the actual command run by `entr`; that is, any `args` specified by the user would be effectively ignored.

In order to make `entr` usable without a shell, this extension uses [a simple binary](/restart_process/tilt-restart-wrapper.go) that invokes `entr` and writes to its stdin.

Note: ideally `entr` could accept files-to-watch via flag instead of stdin, but (for a number of good reasons) this feature isn't likely to be added any time soon (see [entr#33](https://github.com/eradman/entr/issues/33)).

## For Maintainers: Releasing
If you have push access to the `tiltdev` repository on DockerHub, you can release a new version of the binaries used by this extension like so:
1. run `release.sh` (builds `tilt-restart-wrapper` from source, builds and pushes a Docker image with the new binary and a fresh binary of `entr` also installed from source)
2. update the image tag in the [Tiltfile](/restart_process/Tiltfile) with the tag you just pushed (you'll find the image referenced in the Dockerfile contents of the child image--look for "FROM tiltdev/restart-helper")
