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

Download the latest Tilt release from
[GitHub](https://github.com/windmilleng/tilt/releases). Read the [Installation Guide](install.html) for details and prerequistes.

## Describe Your Workflow

Tilt uses your existing Docker/Kubernetes configuration, connected with a simple and powerful description:

```python
# Example Tiltfile for a k8s app with two microservices

# Deploy: tell Tilt what yaml to deploy
k8s_yaml('app.yaml')

# Build: tell Tilt what images to build from which directories
docker_build('companyname/frontend', 'frontend')
docker_build('companyname/backend', 'backend')
```

Setup Tilt in 15 minutes with the [Tutorial](tutorial.html).

## See More
Stop playing 20 questions with `kubectl`. Tilt's UI pulls relevant data to the surface, automatically.

You fix faster when you know what's broken.

## Community

Questions? Comments? Just want to say hi? Find us on the Kubernetes slack. Get an invite at [slack.k8s.io](http://slack.k8s.io) and find
us in [the **#tilt** channel](https://kubernetes.slack.com/messages/CESBL84MV/).

We tweet [@windmill_eng](https://twitter.com/windmill_eng) and
blog about building Tilt at [medium.com/windmill-engineering](https://medium.com/windmill-engineering).

Tilt is Open Source, developed on [GitHub](https://github.com/windmilleng/tilt).

We expect everyone in our community (users, contributors, and employees alike) to abide by our [**Code of Conduct**](code_of_conduct.html).

```eval_rst

.. toctree::
  :hidden:

  self

.. toctree::
   :maxdepth: 1
   :caption: Getting Started

   install
   tutorial

.. toctree::
   :maxdepth: 1
   :caption: Using Tilt

   tiltfile_concepts
   fast_build
   api

.. toctree::
   :maxdepth: 1
   :caption: Appendix

   skaffold
   helm
   example_projects
   docker_compose_alpha

.. toctree::
   :maxdepth: 1
   :caption: About

   code_of_conduct
   faq
```
