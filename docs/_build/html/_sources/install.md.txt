Tilt Installation Guide
=======================

Tilt is currently available for MacOS and Linux.

You'll also need:

- Docker, to build containers
- Kubectl, to cuddle your cluster
- A local Kubernetes cluster (on MacOS, Docker For Mac works for this!)

On MacOS
--------

- Install [Docker For Mac](https://docs.docker.com/docker-for-mac/install/)

- In the Docker For Mac preferences, click [Enable Kubernetes](https://docs.docker.com/docker-for-mac/#kubernetes)

- Verify that it works by opening a terminal and running

```
$ kubectl config get-contexts
$ kubectl config use-context docker-for-desktop
```

- Install the Tilt binary with:

```
$ curl -L https://github.com/windmilleng/tilt/releases/download/v0.4.0/tilt.0.4.0.mac.x86_64.tar.gz | tar -xzv tilt && \
  sudo mv tilt /usr/local/bin/tilt
```

- Verify that you installed it correctly with:

```
$ tilt version
```

On Linux
--------

- Install [Docker](https://docs.docker.com/install/)

- Setup Docker as [a non-root user](https://docs.docker.com/install/linux/linux-postinstall/).

- Install [kubectl](https://kubernetes.io/docs/tasks/tools/install-kubectl/)

- Install [Minikube](https://github.com/kubernetes/minikube#installation)

- Start Minikube as

```
$ minikube start
```

- Verify that it works by opening a terminal and running

```
$ kubectl cluster-info
```

- Install the Tilt binary with:

```
$ curl -L https://github.com/windmilleng/tilt/releases/download/v0.4.0/tilt.0.4.0.linux.x86_64.tar.gz | tar -xzv tilt && \
    sudo mv tilt /usr/local/bin/tilt
```

- Verify that you installed it correctly with:

```
$ tilt version
```

From Source
-----------

If you'd prefer to install `tilt` from source,

- Install [go 1.11](https://golang.org/dl/). Make sure the Go install directory
(usually `$HOME/go/bin`) is on your `$PATH`.

- Run:

```
$ go get -u github.com/windmilleng/tilt/cmd/tilt
```

Verify that you installed it correctly with:

```
$ tilt version
v0.0.0-dev, built 2018-12-21
```

Next Steps
----------

You're ready to start using Tilt! Try it out with [an example project](first_example.html).


