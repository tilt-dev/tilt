# Tutorial: First 15 Minutes
This tutorial walks you through setting up Tilt for your project. It should take 15 minutes, and assumes you've already [installed Tilt](install.html). Before you begin, you may want to join the `#tilt` channel in [Kubernetes Slack](http://slack.k8s.io) for technical and moral support.

Start by `cd`'ing into a project you can already build and deploy to Kubernetes.

## Example Tiltfile
At the end of this guide, your Tiltfile will look something like this:
```python
# Deploy: tell Tilt what YAML to deploy
k8s_yaml('app.yaml')

# Build: tell Tilt what images to build from which directories
docker_build('companyname/frontend', 'frontend')
docker_build('companyname/backend', 'backend')
# ...

# Watch: tell Tilt how to connect locally (optional)
k8s_resource('frontend', port_forwards=8080)
```

## Hello World
Run `tilt up` to enter Tilt's Heads-Up Display. Instead of writing your configuration all at once, we'll use Tilt interactively. Each time you save your configuration, Tilt will reexecute it.


Right now, Tilt should be complaining there's no file named `Tiltfile`. Open it in your editor and write:
```python
print('Hello Tiltfile')
```

Now save the file. Congrats, you just ran your first `Tiltfile`. Tilt's configurations are programs in [Starlark](https://github.com/bazelbuild/starlark#tour>), a dialect of Python. Can you see "Hello Tiltfile" in Tilt's UI? Tilt is also warning you there are no declared resources. Let's add some.

## Step 1: Deploy
The function `k8s_yaml` registers Kubernetes objects you want to deploy:
```python
k8s_yaml('app.yaml')
```

Tilt supports many deployment configuration practices (for more details, check out the [Deploy](tiltfile_concepts.html#deploy) section of "Tiltfile Concepts"):
```python
# multiple YAML files; can be either a list or multiple calls
k8s_yaml(['foo.yaml', 'bar.yaml'])

# run a command to generate YAML
k8s_yaml(local('gen_k8s_yaml.py')) # a custom script
k8s_yaml(kustomize('config_dir')) # built-in support for popular tools
k8s_yaml(helm('chart_dir'))
```

Use the pattern that matches your project (if you're not sure, feel free to ask in [Slack](index.html#community)). You can see when it works because Tilt will display the registered objects.

## Step 2: Build
The function `docker_build` tells Tilt how to build a container image. Tilt automatically builds the image, injects the ID into Kubernetes objects and deploys. (The [Build](tiltfile_concepts.html#build) section of "Tiltfile Concepts" describes optional arguments.)

```python
# docker build -t companyname/frontend ./frontend
docker_build('companyname/frontend', 'frontend')
```

 Try editing a source file; you should see Tilt automatically build and deploy as soon as you save. Add additional images; you should have one `docker_build` call for each container image you're developing.

## Step 3: Watch (Optional)
Tilt can give you consistent port forwards to running pods (whether they're running locally or in the cloud). Call the `k8s_resource` function with the name of the resource you want to access (taken from the UI):
```python
k8s_resource('frontend', port_forwards='9000')
```

You can also use `k8s_resource` to change the resource grouping, or forward multiple ports. Cf. the [Resources](tiltfile_concepts.html#resources) section of `Tiltfile Concepts`.

## Congrats
Tilt is now setup for your project. Try exploring Tilt's UI (there's a context-sensitive legend in the bottom right). Introduce a build error and then a runtime crash; see Tilt's UI respond and surface the relevant problem.

Now's a great time to file bugs, feature requests, or [talk to us](index.html#community) about your use case. If you say hi in the `#tilt` channel of Kubernetes slack we can find time to chat.

You can read in more detail about [Tiltfile Concepts](tiltfile_concepts.html) or [Optimizing your Builds](fast_build.html).
