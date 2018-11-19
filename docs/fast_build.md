Expanding a Tiltfile
=====================

This tutorial looks at a `Tiltfile` with build optimizations.
We explain what they do, and why you would want to use them.

In [the previous tutorial](first_config.md), we introduced the `static_build()` function.
This function builds a Docker image. Tilt will watch the inputs to the
image, and rebuild it every time they change.

This works well for interpreted languages like JavaScript and Python
where you can add the files and go. For servers that need to be compiled,
it would be too slow to recompile from scratch every time.

That's why Tilt has a function `start_fast_build()` for lightning-fast local
Kubernetes development.

Let's look at an example in the [tiltdemo repo](https://github.com/windmilleng/tiltdemo):

```
git clone https://github.com/windmilleng/tiltdemo
cd tiltdemo
```

The `Tiltfile` at the root of the repo contains this function:

```
# -*- mode: Python -*-

def tiltdemo1():
  yaml = read_file('deployments/demoserver1.yaml')
  image_name = 'gcr.io/windmill-test-containers/tiltdemo/demoserver1'
  start_fast_build('Dockerfile', image_name, '/go/bin/demoserver1')
  repo = local_git_repo('.')
  add(repo.path('cmd/demoserver1'),
      '/go/src/github.com/windmilleng/tiltdemo/cmd/demoserver1')
  run('go install github.com/windmilleng/tiltdemo/cmd/demoserver1')
  image = stop_build()
  return k8s_service(image, yaml)
```

This looks similar to the `Tiltfile` in previous tutorials, but instead of building
with `static_build()`, it contains `start_fast_build()` and `stop_build()`. Let's zoom
in on that part of the function.


```
start_fast_build('Dockerfile', image_name, '/go/bin/demoserver1')
repo = local_git_repo('.')
add(repo.path('cmd/demoserver1'),
    '/go/src/github.com/windmilleng/tiltdemo/cmd/demoserver1')
run('go install github.com/windmilleng/tiltdemo/cmd/demoserver1')
image = stop_build()
```

These lines configure `tilt` to do incremental image builds. We'll step through it line-by-line.

* `start_fast_build('Dockerfile', image_name, '/go/bin/demoserver1')`

`start_fast_build` begins the build.
This is setting up the build environment before we add any code.
We build on top of the image in `Dockerfile`. Our new
image has the name in `image_name` and has an entrypoint `/go/bin/demoserver`.

Here's what's in `Dockerfile`:

```
FROM golang:1.10
```

It's only one line! This line says we're starting in a golang:1.10 container.

This `Dockerfile` looks different than most `Dockerfile`s because
it cannot contain any `ADD` or `COPY` lines.
It's only for setting up the environment, not for adding your code.

* `repo = local_git_repo('.')`

The `local_git_repo` function reads the git repo at the given path,
and assigns the repo object to a variable. This object has methods for working with Git repos.

When you're in a Git repo, Tilt knows that it can ignore the `.git` directory.

* add(repo.path('cmd/demoserver1'), '/go/src/github.com/windmilleng/tiltdemo/cmd/demoserver1')

The `add` function copies a directory from outside your container to inside of your container.

In this case, we copy the directory `cmd/demoserver` inside of our Git repo into
the container filesystem.

While Tilt is running, it watches all files in cmd/demoserver. If they change, it copies the file
into the container.

* `run('go install github.com/windmilleng/tiltdemo/cmd/demoserver1')`

The `run` function runs shell commands inside your container.

Every time a file changes, Tilt will run this command again.

One of the major build optimizations that Tilt does is to keep the container running, and
start the command inside the running container.

This is much closer to how we normally run commands for local development. Real humans
don't delete all their code and re-clone it from git every time we need to do a new build!
We re-run the command in the same directory. Modern tools then take advantage of local caches.
Tilt runs commands with the same approach, but inside a container.

* `image = stop_build()`

This line completes the build process and stores the Docker image in the variable `image`.

Next Steps
----------

In this guide, we explored just a few of the functions we can use in a `Tiltfile`
to keep your build fast. For even more functions and tricks,
read the complete [Tiltfile API reference](api.html).
