# Tutorial

This tutorial walks you through setting up Tilt for your project. It should take 15 minutes, and assumes you've already [installed Tilt](install.html). Now's a good time to join the `#tilt` channel in Kubernetes Slack for technical or moral support.

Start in the directory of a project you currently build and deploy to Kubernetes or Docker Compose. If your project uses [skaffold](skaffold.html), [docker-compose](docker-compose.html) or [helm](helm.html) also open that supplement to this Tutorial. If you want to try Tilt but don't have a current Project, you can try one of our [example projects](example_projects.html).

## Example Tiltfile
At the end of this guide, your Tiltfile will look something like this:
```python
# Deploy: tell Tilt what yaml to deploy
k8s_yaml('app.yaml')

# Build: tell Tilt what images to build from which directories
docker_build('companyname/frontend', 'frontend')
docker_build('companyname/backend', 'backend')
# ...

# Watch: tell Tilt how to connect locally (optional)
k8s_resource('frontend', port_forwards=8080)
```

## Hello World
Run `tilt up` to enter Tilt's Heads-Up Display. Instead of writing your configuration all at once, we'll use Tilt interactively. Each time you save your configuration, Tilt will reexecute it. Tilt should be complaining there's no file named `Tiltfile`. Open it in your editor and write:
```python
print('Hello Tiltfile')
```

Now save it. Congrats, you just ran your first `Tiltfile`. Tilt's configurations are programs in Starlark, a dialect of Python. Can you see "Hello Tiltfile" in Tilt's UI? Tilt is also warning you there are no declared resources. Let's add some.

## Step 1: Deploy
Use the `k8s_yaml` function to tell Tilt about Kubernetes objects to deploy:
```python
k8s_yaml('app.yaml')
```

Tilt supports many deployment configuration practices (for more details, check out the YAML section of "Tiltfile Functions"):
```python
# multiple yaml files; can be either a list or multiple calls
k8s_yaml(['foo.yaml', 'bar.yaml'])

# run a command to generate yaml
k8s_yaml(local('gen_k8s_yaml.py')) # a custom script
k8s_yaml(kustomize('config_dir')) # built-in support for popular tools
k8s_yaml(helm('chart_dir'))
```

Add code that calls `k8s_yaml`. Tilt will parse the yaml, display the found objects, and deploy them. If there are problems, update your configuration and let Tilt reexecute your Tiltfile until you see the right objects.

## Step 2: Build
Tilt can build docker images, then inject them into the Kubernetes yaml and deploy. Use the `docker_build` function to tell Tilt how to build an image: (See the Build section of "Tiltfile Functions" for optional args like Dockerfile or build args)

```python
# docker build -t companyname/frontend ./frontend
docker_build('companyname/frontend', 'frontend')
```

Edit some source code and save. Tilt starts rebuilding and will redeploy, automatically. Explore Tilt's UI (there's a legend in the bottom right; as you navigate it changes to tell you what's available from your current state). Try introducing a build error or a runtime crash and see Tilt respond.

You can optimize your builds in various ways. This especially helps for projects with slow builds that run in the a cloud cluster, but we suggest setting up your whole project first.

## Step 3: Watch (Optional)
Tilt can give you consistent port forwards to running pods (whether they're running locally or in the cloud). Call the `k8s_resource` function with the name of the resource you want to access (taken from the UI):
```python
k8s_resource('frontend', port_forwards='9000')
```

You can also use `k8s_resource` to change the resource grouping, or forward multiple ports, as described in the "Watch" section of `Tiltfile Functions`.

## Next Steps
You should now have Tilt working with your project. Next you can:
* Go happily use Tilt
* Let us know how it went (great, bad, or improvement ideas)
* Read more about Tilt
* Learn how to help your team adopt Tilt
* Optimize your Tilt