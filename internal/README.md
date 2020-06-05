# Tilt Architecture

A short guide to the internal code architecture of Tilt.

## Overview

Tilt is fundamentally a control loop with 4 pieces:

- A central `EngineState` that represents everything Tilt "knows".
- A `Store` that mediates changes to `EngineState`
- Subscribers that read from the state store to do stuff.
- Actions that modify the state store.

### The Control Loop

After a state change, the state store calls `OnChange` on each subscriber. Each
subscriber diffs the state to see what changed since the subscriber last looked
at the state.

Depending on what changed, the subscriber may kick off some work, or start
watching an external system (like a Kubernetes Pod) for changes.  As the
subscriber does work, the subscriber creates actions and calls
`store.Dispatch(action)`.

These actions represent state changes. The state store applies these actions to
the state. When it's done applying actions, it calls `OnChange` again, and the
process starts over.

### Examples

Almost everything in Tilt is implmented as a subscriber that fires actions.

- **The pod log manager.** On each `OnChange`, the pod log manager looks for
  new pods that Tilt knows about. If a new pod appears, the log manager
  starts streaming it's logs. As new logs come in, it dispatches actions
  to send the logs to the engine state.
  
- **The Tilt browser UI.** When your browser visits `localhost:10350`,
  Tilt creates a new subscriber that's tied to a WebSocket connection.
  On each `OnChange`, Tilt pushes status updates to the browser
  on the WebSocket.
  
- **The file watcher.** On each `OnChange`, the file watcher looks at all the file
  paths in the state store that we expect to trigger updates. It diffs these
  against all the file watches it's currently managing, then creates new watches
  and deletes unneeded watches. Each watch uses OS APIs to watch the file system,
  then converts those file system notifications into actions to dispatch.

### Concurrency

A subscriber's `OnChange` method is synchronized. The state store will not call
`OnChange` again until the previous `OnChange` has finished.

When a subscriber calls `Dispatch(action)`, that action is put into a FIFO
queue, and the call returns immediately.

The state store typically processes actions in a batch after a backoff period,
updating the state to reflect what's in the actions.

As a corollary, subscribers should not expect an OnChange() call for every
action. For example, if a pod is created then deleted quickly, the actions that
notify the state store of the create and delete might get grouped together, and
the subscribers will never see the pod!

## Package structure

### Core Packages

Here are the core Tilt packages that manage the control loop:

- [store](store) - The central `EngineState`, `Store`, `Subscriber`, and `Action` components

- [engine](engine) - Functions that translate actions into state changes, and registration logic
  for subscribers.
  
- [model](../../pkg/model) - Data models that are shared by the `store` package and other client libraries.

### Subscriber Packages

Most subscribers are subpackages of `engine`. Here's a partial listing of important ones:

- [engine/buildcontrol](engine/buildcontrol) - Decides when to build images and
  deploy resources

- [engine/configs](engine/configs) - Decides when to re-execute the Tiltfile

- [engine/fswatch](engine/fswatch) - Sets up file system watches, and dispatches
  actions when files have changed

- [engine/k8swatch](engine/k8swatch) - Watches Kuberentes resources, and
  dispatches actions when objects (like pods and events) are created, updated,
  or deleted.
  
- [engine/local](engine/local) - Manages servers run by `local_resource(serve_cmd)`

- [engine/portforward](engine/portforward) - Sets up port-forwarding to Kubernetes pods.

- [engine/runtimelog](engine/runtimelog) - Streams logs from Kubernetes and
  Docker Compose containers.
  
There are a few major subscribers that aren't subpackages of `engine`.

- [hud](hud) - The termbox user interface

- [hud/server](hud/server) - Creates websocket connections to send updates to the browser.
  
### Client packages

Almost all other packages in Tilt are client libraries that abstract over
external services that subscribers need to interact with. Here's a partial listing:

- [docker](docker) - An abstraction over the Docker client library.

- [build](build) - An abstraction over docker build, with helpers for building
  docker context tarballs.
  
- [tiltfile](tiltfile) - Executes Tiltfiles, and implements all Tiltfile functions.

- [k8s](k8s) - An abstraction over Kubernetes' client-go.

- [watch](watch) - An abstraction over OS file APIs (like FSEvents and inotify).

- [rty](rty) - A termbox rendering library.

### User interfaces

- [cmd/tilt](../cmd/tilt) - The "main" entrypoint for the CLI.

- [cli](cli) - All Tilt CLI commands.

- [web/src](../web/src) - The Tilt web interface.

