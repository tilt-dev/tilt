Tilt User Guide
====================================

Stop fiddling with the bespoke hand-rolled environment on your local machine.

Write code directly on Kubernetes and leverage modern server debugging tools.

Tilt helps you debug your microservice app and dig into what's wrong.

Overview
--------

Run ``tilt up`` to start working on your services in a clean, dedicated environment.

Tilt watches what you're working on so that it can bring your environment
up-to-date in real-time.

The screencast below demonstrates what a typical Tilt session looks like:
starting two servers, making changes to them, and seeing the changes immediately
take effect (for better or worse)!

.. raw:: html

   <script id="asciicast-211635" src="https://asciinema.org/a/211635.js" async></script>

Install Tilt
------------

Download the Tilt binary on `the github releases page <https://github.com/windmilleng/tilt/releases>`_.

Tilt expects you to already have Docker and a local ``kubectl`` client installed.
Read the more detailed `Installation Guide <quickstart.html>`_ to help you get started quickly.

Configure Your Project for Tilt Development
-------------------------------------------

Down with YAML!

Configure Tilt with a ``Tiltfile``, written in a small subset of Python called
`Starlark <https://github.com/bazelbuild/starlark#tour>`_.

To get started, check out some of the examples on the left nav or dive into the `API reference <api.html>`_.

Next Steps
----------

.. toctree::
   :maxdepth: 1

   quickstart
   first_example
   first_config
   fast_build
   api
