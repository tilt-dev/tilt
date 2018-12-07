A Simple Tiltfile
===================

This tutorial looks at a simple Tiltfile, and breaks down line-by-line what it does.

We'll be digging deeper into the `oneup` project introduced in [the last tutorial](first_example.html).

Let's look at the Tiltfile line-by-line and see what each part does.

```
# -*- mode: Python -*-

docker_build('gcr.io/windmill-test-containers/integration/oneup', '.')
k8s_resource('oneup', 'oneup.yaml')
```

- `# -*- mode: Python -*-`

The first line of the Tiltfile tells IDEs and other tools (like Github fileview)
to use Python syntax highlighting.

A `Tiltfile` uses a small subset of Python called
[Starlark](https://github.com/bazelbuild/starlark#tour). Most Python editors
will work well for editing Tiltfiles.

- `docker_build('gcr.io/windmill-test-containers/integration/oneup', '.')`

This line builds the image and assigns the tag `gcr.io/windmill-test-containers/integration/oneup` to it. The steps to build the image are in the `Dockerfile`.

When we're doing local development, it doesn't matter that much what the image tag is, as long as it matches a name in our Kubernetes YAML.

(When we're doing remote development, the image tag is a URL that tells the cluster where to upload
and download your image).

If we open the Dockerfile, we see

```
FROM golang:1.11
WORKDIR /go/src/github.com/windmilleng/integration/oneup
ADD . .
RUN go install github.com/windmilleng/integration/oneup
ENTRYPOINT /go/bin/oneup
```

If you don't know Go, that's OK. These are steps to run to build a Go server.

- `k8s_resource('oneup', 'oneup.yaml')`

This next line reads Kubernetes YAML, gives it a name and creates it in Kubernetes. Tilt tracks dependencies; you can edit YAML, Dockerfiles or the Tiltfile and Tilt will automatically rebuild your server.

At the risk of diving too deep, let's unpack that YAML file.

```
apiVersion: v1
kind: Namespace
metadata:
  name: tilt-integration
  labels:
    name: tilt-integration
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: oneup
  namespace: tilt-integration
  labels:
    app: oneup
spec:
  selector:
    matchLabels:
      app: oneup
  template:
    metadata:
      labels:
        app: oneup
    spec:
      containers:
      - name: oneup
        image: gcr.io/windmill-test-containers/integration/oneup
        command: ["/go/bin/oneup"]
        ports:
        - containerPort: 8000
```

There's a lot of YAML here! But the idea is easy to summarize: schedule 1 server on Kubernetes.

Next Steps
----------

That's it! In this guide, we've stepped through every line of a simple Tiltfile.

A good way to learn how to use Tilt is run `tilt up`, then make edits to the
Tiltfile, Kubernetes YAML, or Dockerfile. The Tilt UX will update in real-time
in response to your changes.

In the next guide, we'll look at [optimizing a Tiltfile](fast_build.html)
to make your development loop lightning-fast.
