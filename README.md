# Tilt

<img src="assets/logo-wordmark.png" width="250">

[![Build Status](https://circleci.com/gh/tilt-dev/tilt/tree/master.svg?style=shield)](https://circleci.com/gh/tilt-dev/tilt)
[![GoDoc](https://godoc.org/github.com/tilt-dev/tilt?status.svg)](https://pkg.go.dev/github.com/tilt-dev/tilt)

Kubernetes for Prod, Tilt for Dev

Modern apps are made of too many services. They're everywhere and in constant
communication.

[Tilt](https://tilt.dev) powers multi-service development and makes sure they behave!
Run `tilt up` to work in a complete dev environment configured for your team.

Tilt automates all the steps from a code change to a new process: watching
files, building container images, and bringing your environment
up-to-date. Think `docker build && kubectl apply` or `docker-compose up`.

## Watch: Tilt in Two Minutes

[![screencast](assets/demothumb.png)](https://www.youtube.com/watch?v=oSljj0zHd7U)

## Install Tilt

Installing the `tilt` binary is a one-step command.

### macOS/Linux

```bash
curl -fsSL https://raw.githubusercontent.com/tilt-dev/tilt/master/scripts/install.sh | bash
```

### Windows

```powershell
iex ((new-object net.webclient).DownloadString('https://raw.githubusercontent.com/tilt-dev/tilt/master/scripts/install.ps1')
```

For other installation options, see the [Installation Guide](https://docs.tilt.dev/install.html).

## Run Tilt

**New to Tilt?** Our tutorial will [get you started](https://docs.tilt.dev/tutorial.html).

**Configuring a Service?** We have best practice guides for 
[HTML](https://docs.tilt.dev/example_static_html.html), 
[NodeJS](https://docs.tilt.dev/example_nodejs.html), 
[Python](https://docs.tilt.dev/example_python.html), 
[Go](https://docs.tilt.dev/example_go.html),
[Java](https://docs.tilt.dev/example_java.html),
and [C#](https://docs.tilt.dev/example_csharp.html).

**Optimizing a Tiltfile?** Search for the function you need in our 
[complete API reference](https://docs.tilt.dev/api.html).

## Don’t Tilt Alone, Take This

[![Tilt Cloud](assets/TiltCloud-illustration.svg)](https://docs.tilt.dev/snapshots.html)

Are you seeing an error from a server that you don't even work on?

With Tilt Cloud, create web-based interactive reproductions of your local cluster’s state.

Save and share [a snapshot](https://docs.tilt.dev/snapshots.html) with your team
so that they can dig into the problem later. A snapshot lets you explore the
status of running services, errors, logs, and more.

## Community & Contributions

**Questions and feedback:** Join [the Kubernetes slack](http://slack.k8s.io) and
 find us in the [#tilt](https://kubernetes.slack.com/messages/CESBL84MV/)
 channel. Or [file an issue](https://github.com/tilt-dev/tilt/issues).

**Contribute:** Check out our [contribution](CONTRIBUTING.md) guidelines.

**Follow along:** [@tilt_dev](https://twitter.com/tilt_dev) on Twitter. Updates
and announcements on the [Tilt blog](https://blog.tilt.dev).

**Help us make Tilt even better:** Tilt sends anonymized usage data, so we can
improve Tilt on every platform. Details in ["What does Tilt
send?"](http://docs.tilt.dev/telemetry_faq.html).

We expect everyone in our community (users, contributors, followers, and employees alike) to abide by our [**Code of Conduct**](CODE_OF_CONDUCT.md).

## License

Copyright 2018 Windmill Engineering

Licensed under [the Apache License, Version 2.0](LICENSE)
