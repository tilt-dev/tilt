# Docker Compose
Tilt supports `docker-compose` as a runtime, which makes it easy to get Tilt's better UX without changing your config or tools. This doc describes how you can get started with a one-line config, which is even simpler than the [Tutorial](tutorial.html) for Kubernetes projects.


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

## Caveats
Our Docker Compose support is newer than and largely separate from our Kubernetes support. You may hit more/different bugs, which we want to fix.

## Use the Tutorial
Now the [Tutorial](tutorial.html) should take 5 minutes to see your project in Tilt's UX.