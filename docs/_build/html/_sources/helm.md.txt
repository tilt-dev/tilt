# Helm Support in Tilt

Tilt has native support for Helm! To run `helm template` on a chart directory from Tilt simply pass the chart directory to the `helm` built-in like so:

```python
yml = helm('path/to/chart')
# The result is just YAML, which you can use throughout Tilt anywhere you would normally use YAML
k8s_yaml(yml)
```

This could also be implemented as a `local` command if you require further customization:

```python
local("helm template path/to/chart")
```

For complete documentation of both `tilt` and `local` visit the [Tilt API reference](api.html).
