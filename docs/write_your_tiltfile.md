# Write Your Tiltfile

A Tiltfile can be broken down in to three things: Kubernetes YAML, Docker images and resources. In this doc we'll step through all three concepts so you can learn to write your own Tiltfiles with gusto!

To start here's an illustrative example that uses most of the features of Tilt.

```python
## Part 1: Kubernetes YAML
k8s_yaml("k8s.yaml")               # one yaml file
k8s_yaml(['foo.yaml', 'bar.yaml']) # list of yaml files
k8s_yaml(kustomize('config_dir'))  # run kustomize to generate yaml
k8s_yaml(local('gen_k8s_yaml.py')) # run a custom command to general yaml

## Part 2: Images
docker_build("companyname/frontend", "frontend") # docker build <path>
docker_build("companyname/backend", "backend", dockerfile="backend/Dockerfile.dev") # docker build with specific Dockerfile
docker_build("companyname/backend", "graphql", args={"target": "local"} # docker build with build args

## Part 3: Resources
k8s_resource("backend", img="companyname/backend/server") # give a resource a name that's different than the base name of the image
k8s_resource("frontend", port_forwards=9000) # connect to a specific local port
```

Let's dig in to each of these sections in detail.

## Part 1: Kubernetes YAML
Start writing your Tiltfile by telling Tilt about all of your Kubernetes YAML. Once you've done that Tilt can start deploying Kubernetes services and help you iterate towards an even friendlier and more shareable development environment.

There are a couple different ways to tell Tilt about your YAML.

You can pass a single file or a list of files to `k8s_yaml`:
```python
k8s_yaml("k8s.yaml")               # one yaml file
k8s_yaml(['foo.yaml', 'bar.yaml']) # list of yaml files
```

You can invoke a function that produces Kubernetes YAML such as `kustomize` or a custom script:
```python
k8s_yaml(kustomize('config_dir'))  # run kustomize to generate yaml
k8s_yaml(local('gen_k8s_yaml.py')) # run a custom command to general yaml
```

Remember that a `k8s_yaml` function call is all you need to have a functioning Tiltfile!

## Part 2: Images
Next tell Tilt about all your Dockerfiles. Tilt builds your Dockerfiles and understands which Dockerfiles belong to which Kubernetes YAML. You tell Tilt to build a Dockerfile like so:

```python
docker_build("companyname/frontend", "frontend") # docker build ./frontend
```

`docker_build`s in Tilt require that you provide an image name. We use this information to associate Docker builds with the Kubernetes services that the image is used in.

You can also specify advanced configuration options, such as custom dockerfile paths or build arguments:
```python
docker_build("companyname/backend", "backend", dockerfile="backend/Dockerfile.dev") # docker build with specific Dockerfile
docker_build("companyname/backend", "graphql", args={"target": "local"} # docker build with build args
```

Whenever you change a Dockerfile or any files that gets `ADD`'d to a Docker image Tilt will rebuild the Docker image automatically.

## Part 3: Resources
So you've defined a bunch of Kubernetes YAML and Docker Builds. Many of these are related: chances are your Docker images are used in your Kubernetes services. You can organize these in to Tilt resources to streamline development even further.

For example, Tilt by default takes the last path segment in your image name and uses that as the name of the resource. So for example:

```python
docker_build("companyname/backend/server", ".") # name: server
```

You can rename this using a resource!

```python
k8s_resource("backend", img="companyname/backend/server") # give a resource a name that's different than the base name of the image
```

You can also add a Kubernetes port forward to this resource. It's easy!

```python
k8s_resource("backend", img="companyname/backend/server", port_forwards=9000) # connect to a specific local port
```

## Next steps
That's it! We just covered everything you need to know to get your microservices running locally using Tilt. To see everything you can do with Tilt check out the [Tiltfile API reference](api.html).

