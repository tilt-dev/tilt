# From Docker Compose
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

1. Create a `Tiltfile`
2. Define your service name

In Docker Compose your service name is a key in the services list:

```yaml
services:
  spoonerisms:
```

In Tilt your service name is a function defined in your Tiltfile.

```python
def spoonerisms():

```
3. Set the build context

In Docker Compose you can specify your build context and Dockerfile like so:

```yaml
services:
  spoonerisms:
    build:
      context: ./spoonerisms
      dockerfile: ./spoonerisms/Dockerfile
```

In Tilt you tell us where your Dockerfile is and what your build context is.

```python
  img = static_build("spoonerisms/Dockerfile", "gcr.io/myproject/spoonerisms", context="spoonerisms")
```

We also ask that you name the image, so that we can insert it in to your Kubernetes configuration.

4. Create a simple Kubernetes resource for your service
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

4. Combine your build context and k8s config to create a service
```python
  yaml = read_file('spoonerisms.yaml')
  service = k8s_service(img, yaml=yaml)
```

5. Forward your port
In Docker Compose your service has a `ports` field:

```yaml
services:
  spoonerisms:
    ports:
      - "9006:5000"
```

In Tilt services have a `port_forward` method:

```python
  service.port_forward(9006)
```

6. Return your service!

```python
  return service
```

All in all your `Tiltfile` should now look like this:

```python
def spoonerisms():
  img = static_build("spoonerisms/Dockerfile", "gcr.io/myproject/spoonerisms", context="spoonerisms")
  yaml = read_file('spoonerisms.yaml')
  service = k8s_service(img, yaml=yaml)
  service.port_forward(9006)
  return service
```
