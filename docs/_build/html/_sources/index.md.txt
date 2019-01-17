# Tilt User Guide

Local Kubernetes development with no stress.

[Tilt](https://tilt.build) helps you develop your microservices locally.
Run `tilt up` to start working on your services in a complete dev environment
configured for your team.

Tilt watches your files for edits, automatically builds your container images,
and applies any changes to bring your environment
up-to-date in real-time. Think `docker build && kubectl apply` or `docker-compose`.

The screencast below demonstrates what a typical Tilt session looks like:
starting multiple microservices, making changes to them, and seeing any new errors
or logs right in your terminal.

```eval_rst
.. raw:: html

   <p><iframe width="560" height="315" src="https://www.youtube.com/embed/MGeUUmdtdKA" frameborder="0" allow="accelerometer; autoplay; encrypted-media; gyroscope; picture-in-picture" allowfullscreen></iframe></p>
```

## Install Tilt

Download the Tilt binary on
[the github releases page](https://github.com/windmilleng/tilt/releases).

Tilt expects that you already have Docker and `kubectl` installed.
Read the more detailed [Installation Guide](install.html)
to help you `tilt up` quickly.

## Configure Your Workflow to Share With Your Team

Tilt uses your existing `Dockerfiles` and Kubernetes `yaml`, configured in a `Tiltfile`. Here's an example Tiltfile:

```python
# Deploy: tell Tilt what yaml to deploy
k8s_yaml('app.yaml')

# Build: tell Tilt what images to build from which directories
docker_build('companyname/frontend', 'frontend')
docker_build('companyname/backend', 'backend')
```

Our [Tutorial](tutorial.html) takes 15 minutes and walks you through setting up Tilt for your project.

## Community

Questions? Comments? Just want to say hi?

Find us on the Kubernetes slack. Get an invite at [slack.k8s.io](http://slack.k8s.io) and find
us in [the **#tilt** channel](https://kubernetes.slack.com/messages/CESBL84MV/).

We tweet [@windmill_eng](https://twitter.com/windmill_eng) and
blog about building Tilt at [medium.com/windmill-engineering](https://medium.com/windmill-engineering).

We expect everyone in our community (users, contributors, and employees alike) to abide by our [**Code of Conduct**](code_of_conduct.html).

```eval_rst

.. toctree::
   :maxdepth: 1
   :caption: Getting Started

   install
   tutorial

.. toctree::
   :maxdepth: 1
   :caption: Configs From Other Tools

   upgrade_skaffold
   upgrade_docker_compose
   docker_compose_alpha
   helm

.. toctree::
   :maxdepth: 1
   :caption: Using Tilt

   tiltfile_concepts
   fast_build
   api

.. toctree::
   :maxdepth: 1
   :caption: About

   code_of_conduct
   faq
```
