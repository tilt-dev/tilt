# Write Your Tiltfile

A Tiltfile can be broken down in to three things: Kubernetes YAML, Docker images
and resources.  In this doc we'll step through all three concepts so you can
learn to write your own Tiltfiles with gusto!

To start here's an illustrative example that uses most of the features of Tilt.

```python
## Part 1: Kubernetes YAML

# one yaml file
k8s_yaml("k8s.yaml")

# list of yaml files
k8s_yaml(['foo.yaml', 'bar.yaml'])

# run kustomize to generate yaml
k8s_yaml(kustomize('config_dir'))

# run a custom command to generate yaml
k8s_yaml(local('gen_k8s_yaml.py'))

## Part 2: Images

# docker build ./frontend
docker_build("companyname/frontend", "frontend")

# docker build with specific Dockerfile
docker_build("companyname/backend", "backend", dockerfile="backend/Dockerfile.dev")

# docker build with build args
docker_build("companyname/graphql", "graphql", build_args={"target": "local"})

## Part 3: Resources

# give a resource a name that's different than the base name of the image
k8s_resource("backend", image="companyname/backend/server")

# connect to a specific local port
k8s_resource("frontend", port_forwards=9000)
```

Let's dig in to each of these sections in detail.

## Part 1: Kubernetes YAML
Start by telling Tilt about your Kubernetes YAML:

```python
# one yaml file
k8s_yaml("k8s.yaml")
```

Now Tilt will deploy any Kubernetes objects defined in the YAML. A `k8s_yaml`
function call is all you need to have a functioning Tiltfile.

Sometimes you organize your YAML in to multiple files or generate it via a
script. Tilt supports those cases too:

```python
# list of yaml files
k8s_yaml(['foo.yaml', 'bar.yaml'])

# run kustomize to generate yaml
k8s_yaml(kustomize('config_dir'))

# run a custom command to general yaml
k8s_yaml(local('gen_k8s_yaml.py'))
```

## Part 2: Images

Next tell Tilt about all your Dockerfiles but to get the most out of Tilt you
need to tell it about your Dockerfiles. You tell Tilt to build a Dockerfile like
so:

```python
# docker build ./frontend
docker_build("companyname/frontend", "frontend")
```

Tilt extracts the Kubernetes objects that reference this image in to a new
"Resource", discussed below.

### Options to Docker build

You can also specify advanced configuration options, such as custom Dockerfile
paths or build arguments:

```python
# docker build ./frontend
docker_build("companyname/frontend", "frontend")

# docker build with specific Dockerfile
docker_build("companyname/backend", "backend", dockerfile="backend/Dockerfile.dev")

# docker build with build args
docker_build("companyname/graphql", "graphql", args={"target": "local"})
```

## Part 3: Resources

Tilt automatically defines Tilt Resources. Tilt Resources represent logical
groupings of images and Kubernetes YAML.  Tilt by default takes the last path
segment in your image name and uses that as the name of the resource. So for
example:

```python
# name: server
docker_build("companyname/backend/server", ".")
```

If you want to modify Tilt Resources you can make them explicit.

```python
# by default, resources are named after the basename of the image (here, `server`).
# This gives it a different name (`backend`)
k8s_resource("backend", image="companyname/backend/server")
```

You can also add a Kubernetes port forward to this resource.

```python
# connect to a specific local port
k8s_resource("backend", image="companyname/backend/server", port_forwards=9000)
```

## Next steps

That's it! We just covered everything you need to know to get your microservices
running locally using Tilt.  To see everything you can do with Tilt check out
the [Tiltfile API reference](api.html).
