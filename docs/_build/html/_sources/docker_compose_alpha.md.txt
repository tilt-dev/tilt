# Docker Compose (Alpha)
This doc supplements our [Tutorial](tutorial.html) for projects that currently use Docker Compose. Our Docker Compose support is alpha, so we'd really appreciate feedback.

## Comparison
Tilt provides a better User Experience in two ways:
* Tilt handles filesystem watching and updating without requiring manual actions or hand-rolled scripting.
* Tilt's UI shows you the status at a glance, so errors can't scroll off-screen. You can navigate the UI in your terminal and dig into the logs for just one service. (Tilt also has a global log if you do want the full firehose).

Most documentation describes using Tilt to deploy to Kubernetes, but that would be a large change. For Docker Compose projects, Tilt uses Docker Compose as a backend. This allows you to use your existing configuration, debugging tricks, and muscle memory while getting a better UX.

## Tiltfile for Docker Compose
```python
# point Tilt at the existing docker-compose configuration.
docker_compose("./docker-compose.yml")
```

## What We Mean By "Alpha"
Docker Compose support is our newest feature, and so isn't as well-proven as our Kubernetes support. We think Tilt x Docker Compose is better than Docker Compose, but know there are bugs.

You might want to try the current support if:
* you've been curious about Tilt, but use Docker Compose instead of Kubernetes for local dev
* you're frustrated with Docker Compose's log barf and want to easily find logs and errors per-service
* you're excited about dev tools and trying new things
* you enjoy filing bug reports and helping make software better

It's probably not ready for teams looking for a stable, proven tool.

Additionally, there are certain use cases that the current iteration of Tilt x Docker Compose is more or less suited for:

### What to expect from Tilt x Docker Compose, by your use case

* You use Docker Compose for your entire development flow: it handles both building Docker images and spinning them up in the appropriate containers

You are the ideal user for  Tilt x Docker Compose today.

* You use Docker Compose to spin up images that have been built elsewhere (e.g. you have to run `make build` before you run `docker-compose up`)

Tilt will give you better visibility into your app, but won't update automatically. (We're actively working to support adding builds in a `Tiltfile` that aren't in your Docker Compose configuration).

* You build Docker images via Docker Compose (i.e. you specify `build` in your config file) AND you make use of `MOUNT` / `VOLUME` in your `Dockerfile` or `docker-compose.yml`

Tilt may have bugs that result in spurious rebuilds for files that are mounted in volumes. (There are so many corner cases of mounting that our confidence is low). Please let us know and we'll fix it ASAP.
* Your containers automatically restart after crashes (i.e. you set [container restart policy](https://docs.docker.com/compose/compose-file/#restart) in your config file).

Tilt will work, but may be oversensitive and report too many errors. We'd like to understand your case better, so please reach out.


### Install from Source
We're improving Docker Compose support daily, so your feedback will be more relevant if you [install](install.html) from source instead of using a release.

### Feedback Wanted
If you find bugs, unsupported use cases, or think of features, we'd really like to hear.