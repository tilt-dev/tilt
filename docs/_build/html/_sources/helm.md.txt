# For Helm Users

Tilt has native support for Helm! To run `helm template` on a chart directory from Tilt simply pass the chart directory to the `helm` built-in like so:

```python
yml = helm('path/to/chart')
# The result is just YAML, which you can use throughout Tilt anywhere you would normally use YAML
k8s_yaml(yml)
```

If you use Tilt to build your own Docker image and that image appears in your Helm chart we can automatically redeploy your service when the image changes.

```python
docker_build("companyname/service", ".")
yml = helm('path/to/chart') # the resulting yaml uses the companyname/service image
k8s_resource(yml)
```

`helm` could also be implemented as a `local` command if you require further customization:

```python
local("helm template path/to/chart")
```

For complete documentation of both `helm` and `local` visit the [Tilt API reference](api.html).
