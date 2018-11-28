# From Skaffold
## Before you begin
* [Install Tilt](quickstart.html)
* If you're new to Tilt try stepping through [a Simple Tiltfile](first_config.html) first.

## Differences between Skaffold and Tilt
* Skaffold streams the logs for services it started. Tilt provides a UI right in your console to view all of your running services and other relevant errors in addition to the log view that Skaffold provides.
* Skaffold is configured with a static YAML config. Tilt is configured with a `Tiltfile`, written in a small subset of Python called [Starlark](https://github.com/bazelbuild/starlark#tour>).

## Migrate from Skaffold to Tilt
Let's take a simple `skaffold.yaml` file with one service:

```yaml
apiVersion: skaffold/v1alpha5
kind: Config
build:
  artifacts:
  - image: gcr.io/windmill-public-containers/servantes/snack
    context: . # This is the default if not specified
    docker:
      dockerfile: Dockerfile # This is the default if not specified
deploy:
  kubectl:
    manifests:
      - ./deployments/snack.yaml
```

1. Create a `Tiltfile`
2. Define your service name

In Skaffold services are implicitly named from their Kubernetes config. In Tilt services have names given by a function defined in your Tiltfile:

```python
def snack():
```

3. Set the build context

In Skaffold you can specify your build context and Dockerfile like so:

```yaml
build:
  artifacts:
  - image: gcr.io/windmill-public-containers/servantes/snack
    context: . # This is the default if not specified
    docker:
      dockerfile: Dockerfile # This is the default if not specified
```

In Tilt you tell us where your Dockerfile is and what the build_context is.

```python
  img = static_build("Dockerfile", "gcr.io/windmill-public-containers/servantes/snack", context=".")
```

4. Combine your build context and k8s config to create a service

In Skaffold you specify your Kubernetes YAML under the `manifests` key:

```yaml
deploy:
  kubectl:
    manifests:
      - ./deployments/snack.yaml
```

In Tilt we similarly associate your image and your Kubernetes YAML through the concept of a service:

```python
  yaml = read_file('./deployments/snack.yaml')
  service = k8s_service(img, yaml=yaml)
```

5. Return your service!

```python
  return service
```

All in all your `Tiltfile` should now look like this:

```python
def snack():
  img = static_build("Dockerfile", "gcr.io/windmill-public-containers/servantes/snack", context=".")
  yaml = read_file('./deployments/snack.yaml')
  service = k8s_service(img, yaml=yaml)
  return service
```
