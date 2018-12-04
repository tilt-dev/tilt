# From Skaffold
## Before you begin
* [Install Tilt](install.html)
* If you're new to Tilt try stepping through [a Simple Tiltfile](first_config.html) first.

## Differences between Skaffold and Tilt
* Skaffold streams the logs for all services it started. We found one combined stream of all services difficult to use and understand for anything beyond the most simple apps. Tilt instead provides a UI right in your console to view all of your running services and other relevant errors in addition to the log view that Skaffold provides.
* Skaffold is configured with a static YAML config. Tilt is configured with a `Tiltfile`, written in a small subset of Python called [Starlark](https://github.com/bazelbuild/starlark#tour>).

## Migrate from Skaffold to Tilt
Let's take a simple `skaffold.yaml` file with one service:

```yaml
apiVersion: skaffold/v1alpha5
kind: Config
build:
  artifacts:
  - image: gcr.io/windmill-public-containers/servantes/snack
deploy:
  kubectl:
    manifests:
      - ./deployments/snack.yaml
```

1. Create a `Tiltfile`
2. Tell Tilt about your Dockerfile

In Skaffold you can specify your build context and Dockerfile like so:

```yaml
build:
  artifacts:
  - image: gcr.io/windmill-public-containers/servantes/snack
```

In Tilt you tell us where your Dockerfile is and what the build_context is.

```python
  docker_build('gcr.io/windmill-public-containers/servantes/snack', '.')
```

3. Tell Tilt about your YAML

In Skaffold you specify your Kubernetes YAML under the `manifests` key:

```yaml
deploy:
  kubectl:
    manifests:
      - ./deployments/snack.yaml
```

In Tilt we associate your image and your Kubernetes YAML by image tag.

```python
k8s_resource('snack', 'deployments/snack.yaml')
```

In Skaffold services are implicitly named from their Kubernetes config. In Tilt services have names given by the first argument to the `k8s_resource` function.

All in all your `Tiltfile` should now look like this:

```python
docker_build('gcr.io/windmill-public-containers/servantes/snack', '.')
k8s_resource('snack', 'deployments/snack.yaml')
```
