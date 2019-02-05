Install
=======

Tilt is currently available for MacOS and Linux.

You'll also need:

- Docker, to build containers
- Kubectl, to cuddle your cluster
- A local Kubernetes cluster (on MacOS, Docker For Mac works for this!)


Already use Docker Compose for local dev? You can also use Tilt to [run your existing Docker Compose setup](docker_compose.html), in which case all you need to have installed (besides Tilt) is Docker Compose, and you can ignore Kubernetes-specific instructions on this page.

On MacOS
--------

- Install [Docker For Mac](https://docs.docker.com/docker-for-mac/install/)

- In the Docker For Mac preferences, click [Enable Kubernetes](https://docs.docker.com/docker-for-mac/#kubernetes)

- Verify that it works by opening a terminal and running

```
$ kubectl config get-contexts
$ kubectl config use-context docker-for-desktop
```

### Option A) Installing Tilt with Homebrew (recommended)

```
$ brew tap windmilleng/tap
$ brew install windmilleng/tap/tilt
```

### Option B) Installing Tilt from release binaries

```
$ curl -L https://github.com/windmilleng/tilt/releases/download/v0.7.1/tilt.0.7.1.mac.x86_64.tar.gz | tar -xzv tilt && \
  sudo mv tilt /usr/local/bin/tilt
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
$ curl -L https://github.com/windmilleng/tilt/releases/download/v0.7.1/tilt.0.7.1.linux.x86_64.tar.gz | tar -xzv tilt && \
    sudo mv tilt /usr/local/bin/tilt
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

Verifying
---------

After you install Tilt, verify that you installed it correctly with:

```
$ tilt version
v0.7.1, built 2019-02-05
```

Troubleshooting
---------------

If you have any trouble installing Tilt, look for the error message in the
[Troubleshooting FAQ](faq.html#Troubleshooting).


Next Steps
----------

You're ready to start using Tilt! Try our [Tutorial](tutorial.html) to setup your project in 15 minutes.
