# Tilt User Guide

Local Kubernetes development with no stress.

Tilt helps you develop your microservices locally
without slowing you down or making you play Twenty Questions with `kubectl`.

Questions? Comments? Just want to say hi? [File an issue on GitHub](https://github.com/windmilleng/tilt/issues) or find us on the Kubernetes slack in [**#tilt**](https://kubernetes.slack.com/messages/CESBL84MV/).

## Overview

Run `tilt up` to start working on your services in a complete dev environment
configured for your team.

Tilt watches what you're working on so that it can bring your environment
up-to-date in real-time.

The screencast below demonstrates what a typical Tilt session looks like:
starting multiple microservices, making changes to them, and seeing any new errors
or logs right in your terminal.

```eval_rst
.. raw:: html

   <iframe width="560" height="315" src="https://www.youtube.com/embed/MGeUUmdtdKA" frameborder="0" allow="accelerometer; autoplay; encrypted-media; gyroscope; picture-in-picture" allowfullscreen></iframe>
```

## Install Tilt

Download the Tilt binary on
[the github releases page](https://github.com/windmilleng/tilt/releases).

Tilt expects that you already have Docker and `kubectl` installed.
Read the more detailed [Installation Guide](install.html)
to help you `tilt up` quickly.

## Configure Your Workflow to Share With Your Team

Down with YAML!

Configure Tilt with a `Tiltfile`, written in a small subset of Python called
[Starlark](https://github.com/bazelbuild/starlark#tour).

To get started, check out some [examples](first_example.html) or dive into the
[API reference](api.html).

## Next Steps

```eval_rst
.. toctree::
   :maxdepth: 1

   install
   first_example
   first_config
   write_your_tiltfile
   upgrade_skaffold
   upgrade_docker_compose
   fast_build
   api
```
