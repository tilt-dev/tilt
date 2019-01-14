# From Docker Compose

## Or, try it directly!
If you're feeling adventurous, instead of following the steps below to write a `Tiltfile` that replicates your current Docker Compose setup, you can run Tilt directly on your `docker-compose.yml` file: check out our doc on [Docker Compose Support (Alpha)](docker_compose_alpha.md).
## Before you begin
* [Install Tilt](quickstart.html) and Kubernetes if they are not yet installed.
* If you're new to Tilt try stepping through [a Simple Tiltfile](first_config.html) first.

## Differences between Docker Compose and Tilt
* Docker Compose is configured with a static YAML config. Tilt is configured with a `Tiltfile`, written in a small subset of Python called
[Starlark](https://github.com/bazelbuild/starlark#tour>).
* Docker Compose uses runs services on Docker Machine or Docker Swarm. Tilt runs services natively on Kubernetes.

## Migrate from Docker Compose to Tilt
Let's take a simple `docker-compose.yml` file with one service:

```yaml
version: '3'
services:
  spoonerisms:
    build:
      context: ./spoonerisms
      dockerfile: ./spoonerisms/Dockerfile
    command: node /app/src/index.js"
    volumes:
      - ./spoonerisms:/app
    ports:
      - "9006:5000"
```

- Create a `Tiltfile`
- Create a simple Kubernetes resource for your service

For a Node application it might look like this:
```yaml
# spoonerisms.yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: spoonerisms
  labels:
    app: spoonerisms
spec:
  selector:
    matchLabels:
      app: spoonerisms
  template:
    metadata:
      labels:
        app: spoonerisms
        tier: web
    spec:
      containers:
      - name: spoonerisms
        image: gcr.io/myproject/spoonerisms
        command: ["node", "/app/src/index.js"]
        ports:
        - containerPort: 5000
        resources:
          requests:
            cpu: "10m"
```

- Tell Tilt about your Kubernetes resource
```python
k8s_yaml("spoonerisms.yaml")
```

- Set the build context

In Docker Compose you can specify your Docker build context like so:

```yaml
services:
  spoonerisms:
    build:
      context: ./spoonerisms
```

It's similar in Tilt:

```python
docker_build("gcr.io/myproject/spoonerisms", "./spoonerisms")
```

We also ask that you name the image, so that we can insert it in to your Kubernetes configuration.

- Forward your port
In Docker Compose your service has a `ports` field:

```yaml
services:
  spoonerisms:
    ports:
      - "9006:5000"
```

In Tilt you can add port forwards by naming the resource explicitly with `k8s_resource`:

```python
k8s_resource("spoonerisms", port_forwards="9006")
```

All in all your `Tiltfile` should now look like this:

```python
k8s_yaml("spoonerisms.yaml")
docker_build("gcr.io/myproject/spoonerisms", "./spoonerisms")
k8s_resource("spoonerisms", port_forwards="9006")
```
