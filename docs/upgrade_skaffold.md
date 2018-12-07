# From Skaffold
## Before you begin
* [Install Tilt](install.html)
* If you're new to Tilt try stepping through [a Simple Tiltfile](first_config.html) first.

## Differences between Skaffold and Tilt
* Skaffold streams the logs for all services it started. We found one combined stream of all services difficult to use and understand for anything beyond the most simple apps. Tilt instead provides a UI right in your console to view all of your running services and other relevant errors in addition to the log view that Skaffold provides.
* Skaffold is configured with a static YAML config. Tilt is configured with a `Tiltfile`, written in a small subset of Python called [Starlark](https://github.com/bazelbuild/starlark#tour>).

## Migrate from Skaffold to Tilt
Let's take a relatively simple `skaffold.yaml` file with two services:

```yaml
apiVersion: skaffold/v1alpha5
kind: Config
build:
  artifacts:
  - image: gcr.io/windmill-public-containers/servantes/snack
    context: snack
  - image: gcr.io/windmill-public-containers/servantes/spoonerisms
    context: spoonerisms
deploy:
  kubectl:
    manifests:
      - deployments/snack.yaml
      - deployments/spoonerisms.yaml
```

1. Create a `Tiltfile`
2. Tell Tilt about your YAML

In Skaffold you specify your Kubernetes YAML under the `manifests` key:

```yaml
deploy:
  kubectl:
    manifests:
      - deployments/snack.yaml
      - deployments/spoonerisms.yaml
```

In Tilt we specify Kubernetes YAML with the `k8s_yaml` function.

```python
k8s_yaml(['deployments/snack.yaml', 'deployments/spoonerisms.yaml'])
```

3. Tell Tilt about your Dockerfile

In Skaffold you specify the image tag you want to build and deploy:

```yaml
build:
  artifacts:
  - image: gcr.io/windmill-public-containers/servantes/snack
    context: snack
  - image: gcr.io/windmill-public-containers/servantes/spoonerisms
    context: spoonerisms
```

Similarly in Tilt you specify the image tag as well as the Docker build context (`"."` in this case).

```python
docker_build('gcr.io/windmill-public-containers/servantes/snack', 'snack')
docker_build('gcr.io/windmill-public-containers/servantes/spoonerisms', 'spoonerisms')
```

## That's it!

Now your Tiltfile should look like this:

```python
k8s_yaml(['deployments/snack.yaml', 'deployments/spoonerisms.yaml'])
docker_build('gcr.io/windmill-public-containers/servantes/snack', 'snack')
docker_build('gcr.io/windmill-public-containers/servantes/spoonerisms', 'spoonerisms')
```
