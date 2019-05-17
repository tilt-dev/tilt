# Roadmap

✨ Welcome to the future! ✨

This page is for features that the Tilt team has plans to implement.

(These features are less "Wouldn't that be cool?" or "Bugs we should fix", and more "Yes! We should
implement this thing! The design is non-trivial but we've knocked around a few implementation ideas.")

Ideas are roughly in priority order. Each one should be associated with a GitHub issue
that people can follow for updates.

There should be no more than 5-10 ideas on this page. Anything more than that
means that we're doing a poor job choosing what to focus on.

## Next Up

Features that are well-understood and may even have specs.

### Kubernetes Events Display

`kubectl describe` and `kubectl get events` often have important info when a
resource is failing to deploy for some reason (e.g., out of cpu, missing volume,
missing secret, etc).

Tilt should surface this info in some way.

https://github.com/windmilleng/tilt/issues/1584

### Richer File Ignores for Image Builds

Many Tilt users develop in repos with multiple image builds.

Tilt can be very clunky for them, because a single change forces all the images
to rebuild. They need each image build to ignore large parts of the repo.

https://github.com/windmilleng/tilt/issues/1656

### Manual Update Control

Tilt watches for file changes, and auto-updates your service whenever something
changes. This works well for fast-building services. For slow services, the user
wants to be more conservative about when builds start, or may need to cancel
builds.

Tilt should have a mode where it labels services as dirty in the UX when
files have changed, but does not start building/updating
them until the user presses a key.

https://github.com/windmilleng/tilt.specs/blob/master/manual_update_control.md

https://github.com/windmilleng/tilt/issues/1012

### Working with Image Registries

Kubernetes is moving towards a world where the container runtime and the image
registry are separate services.

Currently, Tilt takes advantage of them being the same on Docker For Mac. We can
build the image in-cluster and it's immediately available in the container
runtime.

If they're separate, the workflow is more complex. The MicroK8s team wrote
a great doc on this problem:

https://github.com/ubuntu/microk8s/blob/master/docs/working.md

Tilt could do a lot more to automate these workflows.

https://github.com/windmilleng/tilt/issues/1621

https://github.com/windmilleng/tilt/issues/1620

## Later

Features that are not immediate concerns, but our on our mid-term radar.

This might change as we collect data from users on how important they are.

### Parallel Builds

A few people have requested the ability to run builds in parallel. A lot of
docker builds do a lot of network operations, where doing the work in parallel
(pulling images, apt installing things) would help a lot.

We expect that this is more than just an engine change. We'll also need to
improve how we show the logs for the interleaving builds, so that the output
isn't too confusing.

https://github.com/windmilleng/tilt/issues/1438

### Unit Tests

Tilt already has the ability to run Kubernetes jobs. How hard would it be to provide a wrapper around jobs for running unit and integration tests? Like regular services they could be built on run when the code included in the image that runs the unit test(s) changes.

You can see a proof-of-concept implementation of this in [Tilt's Tiltfile](https://github.com/windmilleng/tilt/blob/master/Tiltfile).

Open questions:
- Could we parse output in some way to display a better view of which tests failed?
- Could we make it easy to break down one monolithic test run in to many parallel test runs?

### Service Dependencies

There are situations where services need to start in a specific order: redis
needs to start before service A, or service B opens a persistent connection to
service C. Users should be able to express these dependencies in Tilt to ensure
that they are started in the correct order.

Open questions:
- Is it sufficient to only worry about this on start, or are there scenarios where, if service A depends on service B, that we need to restart service A if service B changes?
- Is it more ergonomic to just make Tilt start the services in the order you define them, or is it better to make an explicit dependency graph with some new syntax feature?
