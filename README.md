# Tilt

<img src="assets/logo-wordmark.png" width="250">

[![Build Status](https://circleci.com/gh/windmilleng/tilt/tree/master.svg?style=shield)](https://circleci.com/gh/windmilleng/tilt)
[![GoDoc](https://godoc.org/github.com/windmilleng/tilt?status.svg)](https://godoc.org/github.com/windmilleng/tilt)

Local Kubernetes development with no stress.

[Tilt](https://tilt.dev) helps you develop your microservices locally.
Run `tilt up` to start working on your services in a complete dev environment
configured for your team.

Tilt watches your files for edits, automatically builds your container images,
and applies any changes to bring your environment
up-to-date in real-time. Think `docker build && kubectl apply` or `docker-compose up`.

## Watch: Tilt in Two Minutes

[![screencast](assets/demothumb.png)](https://www.youtube.com/watch?v=oSljj0zHd7U)

## Install Tilt

If you don't know where to start, start here:

[Complete Tilt User Guide](https://docs.tilt.dev/)

Download the Tilt binary on
[the github releases page](https://github.com/windmilleng/tilt/releases).

Tilt expects that you already have Docker and `kubectl` installed. Alternately, you can
skip Kubernetes altogether and run Tilt with your pre-existing `docker-compose.yml`.
Read the more detailed [Installation Guide](https://docs.tilt.dev/install.html)
to help you `tilt up` quickly.

## Configure Your Workflow to Share With Your Team

Configure Tilt with a `Tiltfile`, written in a small subset of Python called
[Starlark](https://github.com/bazelbuild/starlark#tour).

To get started, check out the [tutorial](https://docs.tilt.dev/tutorial.html) or dive into the
[API reference](https://docs.tilt.dev/api.html).

## Community

Questions? Comments? Just want to say hi?

Find us on the Kubernetes slack. Get an invite at [slack.k8s.io](http://slack.k8s.io) and find
us in [the **#tilt** channel](https://kubernetes.slack.com/messages/CESBL84MV/).

We tweet [@tilt_dev](https://twitter.com/tilt_dev) and
blog about building Tilt at [blog.tilt.dev](https://blog.tilt.dev).

We expect everyone in our community (users, contributors, followers, and employees alike) to abide by our [**Code of Conduct**](https://docs.tilt.dev/code_of_conduct.html).

## Development

To make changes to Tilt, read the [developer guide](DEVELOPING.md).

For bugs and feature requests, file an [issue](https://github.com/windmilleng/tilt/issues)
or check out the [feature roadmap](https://github.com/orgs/windmilleng/projects/3).

## Telemetry and Privacy
We're a small company trying to make Tilt awesomer. We can do this better if we understand which features people are using and which bugs people are running into. You can enable sending telemetry data to https://events.windmill.build in the UI or by running `tilt analytics opt in`. It really helps us!

The data is meant to be about your use of Tilt (e.g., which Tiltfile or Web UI features do you use), not collecting data about you or your project. It's possible that some of the data we collect could include snippets of data about your project (e.g. that you have a service named `deathray-backend` or an error message that includes the string it failed to parse). We try to avoid this, but you should probably not opt-in if you're working on a classified project.

We will not resell or give away this data. (Data may be sent to third parties, like Datadog,
but only to help us analyze the data.)

You can change your mind at any time by running `tilt analytics opt <in|out>` and restarting Tilt. Until you make a choice, Tilt will send a minimal amount of data (this helps us improve the installation/opting flow).

Tilt connects to other online services for purposes like finding and downloading product updates and resources.

## License

Copyright 2018 Windmill Engineering

Licensed under [the Apache License, Version 2.0](LICENSE)
