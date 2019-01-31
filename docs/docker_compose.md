# Docker Compose
Tilt can use `docker-compose` to orchestrate your services (instead of Kubernetes). This doc describes how you can get Tilt's UX for your Docker Compose project using the same config and tools plus a one-line Tiltfile. (This is simpler than the config for Kubernetes projects described in the [Tutorial](tutorial.html).)


## Comparison
Tilt provides a better User Experience in two ways:
* Tilt's UI shows you status at a glance, so errors can't scroll off-screen. You can navigate the UI in your terminal and dig into the logs for just one service. (Tilt also has a global log if you do want the full firehose).
* Tilt handles filesystem watching and updating without requiring manual actions or hand-rolled scripting.

Most of our documentation describes using Tilt to deploy to Kubernetes, but that would be a large change. For Docker Compose projects, Tilt uses Docker Compose as a backend. This allows you to use your existing configuration, debugging tricks, and muscle memory while getting a better UX.

## Tiltfile for Docker Compose
```python
# point Tilt at the existing docker-compose configuration.
docker_compose("./docker-compose.yml")
```

## Caveats
Our Docker Compose support is newer than (and largely separate from) Tilt's Kubernetes support. You may hit more/different bugs, which we want to fix -- please file issues or tell us in Slack.

## Docker Compose Under The Hood
Tilt uses Docker Compose to run your services, so you can also use `docker-compose` to examine state outside Tilt.

## Use the Tutorial
Now the [Tutorial](tutorial.html) should take 5 minutes to see your project in Tilt's UX.