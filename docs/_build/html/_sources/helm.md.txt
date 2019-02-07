# Helm Integration
Tilt supports Helm out-of-the-box. The `helm` function runs `helm template` on a chart directory and returns the generated yaml. Use this function in the Deploy step of our [Tutorial](tutorial.html).

```python
k8s_yaml(helm('path/to/chart'))
```

## Further Customization
You could also run `helm` with a call to `local` if you require further customization:

```python
k8s_yaml(local("helm template path/to/chart"))
```

(If you need to do this, let us know: we'd love to extend our built-in support to cover your use case.)
