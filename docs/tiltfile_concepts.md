# Tiltfile Concepts

This doc describes concepts in the Tiltfile, expanding on the intro in [Tilt Your Project](tilt_your_project.html). Unlike the [API Reference](api.html), it groups functions by themes and explains why you'd choose to use a function.

## Execution Model
`Tiltfile`s are written in [Starlark](https://github.com/bazelbuild/starlark), a dialect of Python. Tilt executes the `Tiltfile` on startup.

Functions like `k8s_yaml` and `docker_build` register information. At the end of the execution, Tilt uses the resulting configuration. In addition to the final configuration, Tilt records file accesses. Tilt listens to these files, and reexecutes when one changes (but not on every source file change).

Because Tilt uses a programming language, you can configure it with familiar constructs like loops, functions, arrays, etc. This makes Tilt more extensible than a configuration that requires hard-coding all possible options up-front.

## Deploy
The first function in a `Tiltfile` is generally a call to `k8s_yaml`. You can call `k8s_yaml` in a variety of ways, depending on how your project organizes or generates yaml. Let's look at some alternatives:

```python
# one static yaml file
k8s_yaml('app.yaml')

# multiple yaml files in one call
k8s_yaml(['foo.yaml', 'bar.yaml'])

# multiple yaml files in multiple calls
k8s_yaml('baz.yaml')
k8s_yaml('quux.yaml')

# call out to a built-in tool
k8s_yaml(kustomize('config_dir')) # built-in support for popular tools
k8s_yaml(helm('chart_dir'))
```

Tilt has built-in functions to generate kubernetes yaml with `kustomize` or `helm`. (If you think we're overlooking a popular tool, let us know so we can add it.)

## Custom Commands
If your project uses a custom tool to generate kubernetes yaml, you can still use Tilt. You don't have to wait for us to add support or fork Tilt and implement it yourself. Use the `local` function:
```python
text = local('./foo.py') # runs command foo.py
k8s_yaml(text)
```

`local` runs a command, and returns its `stdout` as a Blob. (A Blob is a string but is interpreted by `k8s_yaml` as text instead of as a filename.) Note: Tilt doesn't know what files a command accesses, so you need to use the function `read_file` to record accesses. If you don't call `read_file`, Tilt won't reexecute the `Tiltfile` when those files change. For example, if `foo.py` depends on the files `config/base.yaml` and `data/versions.txt`:

```python
read_file('config/base.yaml')
read_file('data/versions.txt')
text = local('./foo.py')
k8s_yaml(text)
```

You can also use python features like list comprehensions. For example, if you have a script that generates yaml for one microservice at a time, you could do:

```python
# define a function that returns the config for one microservice
def microservice_config(name):
  # record file access, using python string substitution to generate filename
  read_file('config/%s.yaml' % name)
  # run the script with an argument
  return local('./config/generate.py %s' % name)

# define the service names
services = ['frontend', 'backend', 'users', 'graphql']

# loop over each service and register its config
[k8s_yaml(microservice_config(service)) for service in services]
```

Using `local` judiciously can let you use existing tools with Tilt, without having to rewrite or abandon them immediately.

## Build
The `docker_build` function aims to support most usages of docker. Here's a cheat-sheet that maps docker command lines to a `docker_build` call:

```python
# docker build -t companyname/frontend ./frontend
docker_build("companyname/frontend", "frontend")

# docker build -t companyname/frontend -f frontend/Dockerfile.dev frontend
docker_build("companyname/frontend", "frontend", dockerfile="frontend/Dockerfile.dev")

# docker build -t companyname/frontend --build-arg target=local frontend
docker_build("companyname/frontend", "frontend", build_args={"target": "local"})
```

These optional arguments can of course be combined.

## Resources
Tilt's UI makes it easier to find errors by grouping related status and output. E.g., when you edit a file, you want to know what error it caused, whether it's an error at build-time, deploy-time, or run-time. Tilt calls these groupings "Resources". Each Resource has a line in the UI that can be expanded and investigated.

Tilt generates these groups after executing your `Tiltfile`. We're actively working on how to group in ways that make the most intuitive sense, so the specific algorithm is in-flux. We'll expand this paragraph when it's more settled.

You can configure a resource with a call to `k8s_resource`. Today, the only relevant configuration is the argument `port_forwards`. Tilt supports a few alternatives:

```python
# connect localhost:9000 to the default container port
k8s_resource('frontend', port_forwards=9000)

# connect localhost:9000 to container port 8000
k8s_resource('frontend', port_forwards='9000:8000')

# connect localhost:9000 to container port 8000
# and localhost:9001 to container port 8001
k8s_resource('frontend', port_forwards=['9000:8000', '9001:8001'])
```

You can also use calls to `k8s_resource` to control the grouping of resources. This should only be necessary in extreme cases. Because it's in-flux, please reach out and we'll help you individually if you think this is necessary.

## Summary
Tilt's configuration is a program that connects your existing build and deploy configuration. We've made our functions ergonomic for simple cases and general enough to support a wide range of cases. If you're not sure how to accomplish something, we'd love to either help you find the right way, or add support for a case we've overlooked.