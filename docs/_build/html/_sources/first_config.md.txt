A Simple Tiltfile
===================

This tutorial looks at a simple Tiltfile, and breaks down line-by-line what it does.

We'll be digging deeper into the `oneup` project introduced in [the last tutorial](first_example.html).

Let's look at the Tiltfile line-by-line and see what each part does.

```
# -*- mode: Python -*-

docker_build('gcr.io/windmill-test-containers/integration/oneup', '.')
k8s_resource('oneup', 'oneup.yaml', port_forwards=8100)
```

- `# -*- mode: Python -*-`

The first line of the Tiltfile tells IDEs and other tools (like Github fileview)
to use Python syntax highlighting.

A `Tiltfile` uses a small subset of Python called
[Starlark](https://github.com/bazelbuild/starlark#tour). Most Python editors
will work well for editing Tiltfiles.

- `docker_build('gcr.io/windmill-test-containers/integration/oneup', '.')`

This line builds a docker image. The first argument `gcr.io/windmill-test-containers/integration/oneup` is the tag for the image.
The second argument `.` is the directory to use as the build context. In this case, we use the source code and Dockerfile
in the current directory.

When we're doing local development, it doesn't matter that much what the image tag is, as long as it matches a name in our Kubernetes YAML.

(When we're doing remote development, the image tag is a URL that tells the cluster where to upload
and download your image).

If we open the Dockerfile, we see

```dockerfile
FROM golang:1.11
WORKDIR /go/src/github.com/windmilleng/integration/oneup
ADD . .
RUN go install github.com/windmilleng/integration/oneup
ENTRYPOINT /go/bin/oneup
```

If you don't know Go, that's OK. These are steps to run that build a Go server.

- `k8s_resource('oneup', 'oneup.yaml', port_forwards=8100)`

This next line reads Kubernetes YAML, gives it a name, creates it in Kubernetes, and sets up a localhost:8100 listener.

Tilt tracks dependencies; you can edit YAML, Dockerfiles or the Tiltfile and Tilt will automatically rebuild your server.

At the risk of diving too deep, let's unpack that YAML file.

```yaml
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

The `port_forwards=8100` tells Tilt to connect `localhost:8100` to the main
`containerPort` for the `oneup` container.  Tilt will wait for the server to
come up and make the connection when its ready.

Next Steps
----------

That's it! In this guide, we've stepped through every line of a simple Tiltfile.

A good way to learn how to use Tilt is run `tilt up`, then make edits to the
Tiltfile, Kubernetes YAML, or Dockerfile. The Tilt UX will update in real-time
in response to your changes.

In the next guide, we'll look at [optimizing a Tiltfile](fast_build.html)
to make your development loop lightning-fast.
