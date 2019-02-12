# From Skaffold
Tilt is a great upgrade to [Skaffold](https://skaffold.dev) for local dev. This doc compares Tilt to Skaffold and describes how to translate your configuration, which makes our 15 Minute [Tutorial](tutorial.html) even easier.

## Comparison
* Tilt's UI shows you status at a glance, so errors can't scroll off-screen. You can navigate the UI in your terminal and dig into the logs for just one service. (Tilt also has a global log if you do want the full firehose).
* Tilt's configuration is [Starlark](https://github.com/bazelbuild/starlark#tour>), a subset of Python. This allows simple configs to be shorter and complex configs to be possible.

## Translate Skaffold Configuration
Skaffold concepts map almost directly into Tilt. Let's translate an example Skaffold configuration with two deployments and two images:

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

The corresponding `Tiltfile` is:
```python
k8s_yaml(['deployments/snack.yaml', 'deployments/spoonerisms.yaml'])
docker_build('gcr.io/windmill-public-containers/servantes/snack', 'snack')
docker_build('gcr.io/windmill-public-containers/servantes/spoonerisms', 'spoonerisms')
```
