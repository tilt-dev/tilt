# Write Your Tiltfile

A Tiltfile can be broken down in to three things: Kubernetes YAML, Docker builds and resources. In this doc we'll step through all three concepts so you can write your own Tiltfiles with gusto!

## Kubernetes YAML
The minimal Tiltfile defines one Kubernetes YAML:

```python
k8s_yaml('config/kubernetes.yaml')
```

This tells Tilt about the provided bit of Kubernetes YAML. Tilt will apply this YAML to your Kubernetes cluster.

To explore this concept further lets open up `config/kubernetes.yaml` and see what it contains:

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: secrets
  labels:
    app: secrets
    owner: varowner
spec:
  selector:
    matchLabels:
      app: secrets
      owner: varowner
  template:
    metadata:
      labels:
        app: secrets
        tier: web
        owner: varowner
    spec:
      containers:
      - name: secrets
        image: gcr.io/windmill-public-containers/servantes/secrets
        command: ["/go/bin/secrets"]
        env:
        - name: TEMPLATE_DIR
          value: "/go/src/github.com/windmilleng/servantes/secrets/web/templates"
        - name: THE_SECRET
          valueFrom:
            secretKeyRef:
              name: servantes-stuff
              key: stuff
        ports:
        - containerPort: 8081
        resources:
          requests:
            cpu: "10m"
---
apiVersion: v1
kind: Service
metadata:
  name: secrets
  labels:
    app: secrets
    owner: varowner
spec:
  ports:
  - port: 80
    targetPort: 8081
    protocol: TCP
  selector:
    app: secrets
    owner: varowner
---
apiVersion: v1
kind: Secret
metadata:
  name: servantes-stuff
type: Opaque
data:
  stuff: aGVsbG8gd29ybGQ=
  things: bXktZm9ydHVuZS1zZWNyZXQ=
```

That's a lot of YAML! Luckily for us Tilt will break this down and keep track of it so we don't have to. It will divide this in to three Kubernetes YAML entities internally:

1. The `secrets` deployment
2. The `secrets` service
3. The Kubernetes Secret named `servantes-stuff`

Remember that a `k8s_yaml` function call is all you need to have a functioning Tiltfile!

## Docker Builds
Tilt can build your Dockerfiles and can understand which Dockerfiles belong to which Kubernetes YAML. There are two ways to Docker images in Tilt. Let's start with `docker_build`.

### `docker_build`

You tell Tilt to build a Dockerfile like so:

```python
docker_build('gcr.io/windmill-public-containers/servantes/secrets', '.')
```

`docker_build`s in Tilt require that you provide an image name. We use this information to associate Docker builds with the Kubernetes services that the image is used in. The second parameter is the Docker context that gets passed to Docker. This context is a path to a directory and should contain a `Dockerfile`, but you can always specify your own:

```python
docker_build('gcr.io/windmill-public-containers/servantes/secrets', '.', dockerfile='config/Dockerfile')
```

Whenever you change a Dockerfile or any code that gets `ADD`'d to a Docker image Tilt will rebuild the Docker image automatically.

### `fast_build`

`docker_build` works well for interpreted languages like JavaScript and Python. For servers that need to be compiled, it would be too slow to recompile and rebuild the docker image from scratch every time anything changes. That's why Tilt has a function `fast_build` for lighting-fast development of compiled services.

```python
repo = local_git_repo(.)
fast_build('gcr.io/windmill-public-containers/servantes/secrets', 'Dockerfile.go.base')
  .add(repo.path('secrets'), '/go/src/github.com/windmilleng/servantes/secrets')
  .run('go install github.com/windmilleng/servantes/secrets')
```

These lines configure Tilt to do an incremental Docker build. Let's step through it line-by-line:

1. `repo = local_git_repo(.)`

The local_git_repo function reads the git repo at the given path, and assigns the repo object to a variable. This object [has methods](TODO) for working with Git repos.

When you’re in a Git repo, Tilt knows that it can ignore the `.git` directory.

2. `fast_build('gcr.io/windmill-public-containers/servantes/secrets', 'Dockerfile.go.base')`

This begins the build. We build on top of the image in `Dockerfile.go.base`. Our new image has a name, `gcr.io/windmill-public-containers/servantes/secrets`, just like in `docker_build`. For the record here's what's in `Dockerfile.go.base`:

```Dockerfile
FROM golang:1.10
```

Fast build Dockerfiles cannot contain any ADD or COPY lines. It’s only for setting up the environment, not for adding your code. So this Dockerfile might look different than most.

3. `.add(repo.path('secrets'), '/go/src/github.com/windmilleng/servantes/secrets')`

The add function copies a directory from outside your container to inside of your container.

In this case, we copy the directory cmd/demoserver inside of our Git repo into the container filesystem.

While Tilt is running, it watches all files in cmd/demoserver. If they change, it copies the file into the container.

4. `.run('go install github.com/windmilleng/servantes/secrets')`

The run function runs shell commands inside your container. Every time a file changes Tilt will run this command again.

One of the major build optimizations that Tilt does is to keep the container running, and start the command inside the running container. This is much closer to how we normally run commands for local development!

## Resources
So you've defined a bunch of Kubernetes YAML and Docker Builds. Many of these are related: chances are your Docker images are used in your Kubernetes services. You can organize these in to Tilt resources to streamline development even further.

Here's a Tiltfile with some Kubernetes YAML and Docker Builds defined.

```python
k8s_yaml('config/kubernetes.yaml')
docker_build('gcr.io/windmill-public-containers/servantes/fe', 'fe')
docker_build('gcr.io/windmill-public-containers/servantes/vigoda', 'vigoda')
```

There results in several entities being present in Tilt:

1. `fe`, defined when Tilt noticed that `gcr.io/windmill-public-containers/servantes/fe` exists in `k8s_yaml`.
2. `vigoda`, defined when Tilt noticed that `gcr.io/windmill-public-containers/servantes/vigoda` exists in `k8s_yaml`.
3. The unresourced entities leftover in `k8s_yaml`.

Turning `fe` in to an explicit Tilt resource is easy:

```python
k8s_resource('fe')
```

This isn't very useful on it's own though. However, now that we've made it an explicit Tilt resource we can modify it right from Tilt. Say we wanted to add a Kubernetes port forward to this resource. It's easy!

```python
k8s_resource('fe', port_forwards=9000)
```

You can think of Tilt resources as a way of organizing and manipulating other entities. To see everything you can do with a Tilt resource check out the [resource API documentation](TODO).
