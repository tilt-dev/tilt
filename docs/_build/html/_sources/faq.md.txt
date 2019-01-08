Frequently Asked Questions
==========================

Building Container Images
-------------------------

### Q: All the Tilt examples store the image at `gcr.io`. Isn't it really slow to push images up to Google's remote repository for local development?

You're right, that would be slow!

Most local Kubernetes development solutions let you build images directly inside
the cluster. There's no need to push the image to a remote repository.

When you're using Docker for Mac or Minikube, Tilt will automatically build the
images in-cluster. When it detects this case, it will even modify your
Kubernetes configs to set ImageNeverPull, so that Kubernetes will emit an error
if it even tries to pull an image from a remote server.

### Q: Docker Buildkit is cool! How do I use it?

[BuildKit](https://github.com/moby/buildkit) is a new build engine in
Docker for building container images.

Tilt will automatically enable Buildkit if your local Docker installation
supports it.

BuildKit is supported on Docker v18.06 when Experimental mode is enabled, and on
Docker v18.09+

### Q: How do I tell Tilt to build my images with a remote Docker server?

Tilt reads the same environment variables as the `docker` command for choosing a
server. Specifically:

- `DOCKER_HOST`: Set the url to the docker server.
- `DOCKER_API_VERSION`: Set the version of the API.
- `DOCKER_CERT_PATH`: Set the path to load the TLS certificates from.
- `DOCKER_TLS_VERIFY`: To enable or disable TLS verification when using `DOCKER_CERT_PATH`, off by default.

This is helpful if you have a more powerful machine in the cloud that you want
to build your images.

If you are using Minikube, Tilt will automatically connect to the Docker server
inside Minikube.  This helps performance because Tilt doesn't need to waste time
copying Docker images around.




