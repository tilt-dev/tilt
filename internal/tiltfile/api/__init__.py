from typing import Dict, Union, List, Callable, Any, Optional

# Our documentation generation framework doesn't properly handle __file__,
# so we call it __file__ and edit it later.
file__: str = ""
"""The path of the Tiltfile. Set as a local variable in each Tiltfile as it loads.
"""

class Blob:
  """The result of executing a command on your local system.

   Under the hood, a `Blob` is just a string, but we wrap it this way so Tilt knows the difference between a string meant to convey content and a string indicating, say, a filepath.

   To wrap a string as a blob, call ``blob(my_str)``"""

class LiveUpdateStep:
  """A step in the process of performing a LiveUpdate on an image's container.

  For details, see the `Live Update documentation <live_update_reference.html>`_.
  """
  pass

class PortForward:
  """
  Specifications for setting up and displaying a Kubernetes port-forward.

  For details, see the :meth:`port_forward` method.
  """
  pass


class Probe:
  """Specification for a resource readiness check.

  For details, see the :meth:`probe` function.
  """
  pass


class ExecAction:
  """Specification for a command to execute that determines resource readiness.

  For details, see the :func:`probe` and :func:`exec_action` functions.
  """
  pass


class HTTPGetAction:
  """Specification for a HTTP GET request to perform that determines resource readiness.

  For details, see the :func:`probe` and :func:`http_get_action` functions.
  """
  pass


class TCPSocketAction:
  """Specification for a TCP socket connection to perform that determines resource readiness.

  For details, see the :func:`probe` and :func:`tcp_socket_action` functions.
  """
  pass


def port_forward(local_port: int,
                 container_port: Optional[int] = None,
                 name: Optional[str] = None,
                 link_path: Optional[str] = None,
                 host: Optional[str] = None) -> PortForward:
  """
  Creates a :class:`~api.PortForward` object specifying how to set up and display a Kubernetes port forward.

  By default, the host for a port-forward is ``localhost``. This can be changed with
  the ``--host`` flag when invoking Tilt via the CLI.

  Args:
    local_port (int): the local port to forward traffic to.
    container_port (int, optional): if provided, the container port to forward traffic *from*.
      If not provided, Tilt will forward traffic from ``local_port``, if exposed, and otherwise,
      from the first default container port. E.g.: ``PortForward(1111)`` forwards traffic from
      container port 1111 (if exposed; otherwise first default container port) to ``localhost:1111``.
    name (str, optional): the name of the link. If provided, this will be text of this URL when
      displayed in the Web UI. This parameter can be useful for disambiguating between multiple
      port-forwards on a single resource, e.g. naming one link "App" and one "Debugger." If not
      given, the Web UI displays the URL itself (e.g. "localhost:8888").
    link_path (str, optional): if given, the path at the port forward URL to link to; e.g. a port
      forward on localhost:8888 with ``link_path='/v1/app'`` would surface a link in the UI to
      ``localhost:8888/v1/app``.
    host (str, optional): if given, the host of the port forward (by default, ``localhost``). E.g.
      a call to `port_forward(8888, host='elastic.local')` would forward container port 8888 to
      ``elastic.local:8888``.
  """
  pass

class Link:
  """
  Specifications for a link associated with a resource in the Web UI.

  For details, see the :meth:`link` method.
  """
  pass

def link(url: str, name: Optional[str]) -> Link:
  """
  Creates a :class:`~api.Link` object that describes a link associated with a resource.

  Args:
    url (str): the URL to link to
    name (str, optional): the name of the link. If provided, this will be the text of this URL when displayed in the Web UI. This parameter can be useful for disambiguating between multiple links on a single resource, e.g. naming one link "App" and one "Debugger." If not given, the Web UI displays the URL itself (e.g. "localhost:8888").
  """
  pass

def fall_back_on(files: Union[str, List[str]]) -> LiveUpdateStep:
  """Specify that any changes to the given files will cause Tilt to *fall back* to a
  full image build (rather than performing a live update).

  ``fall_back_on`` step(s) may only go at the beginning of your list of steps.

  (Files must be a subset of the files that we're already watching for this image;
  that is, if any files fall outside of DockerBuild.context or CustomBuild.deps,
  an error will be raised.)

  For more info, see the `Live Update Reference <live_update_reference.html>`_.

  Args:
      files: a string or list of strings of files. If relative, will be evaluated relative to the Tiltfile. Tilt compares these to the local paths of edited files when determining whether to fall back to a full image build.
  """
  pass


def set_team(team_id: str) -> None:
  """Associates this Tiltfile with the `team <teams.html>`_ identified by `team_id`.

  Sends usage information to Tilt Cloud periodically.
  """
  pass

def sync(local_path: str, remote_path: str) -> LiveUpdateStep:
  """Specify that any changes to `localPath` should be synced to `remotePath`

  May not follow any `run` steps in a `live_update`.

  For more info, see the `Live Update Reference <live_update_reference.html>`_.

  Args:
      localPath: A path relative to the Tiltfile's directory. Changes to files matching this path will be synced to `remotePath`.
          Can be a file (in which case just that file will be synced) or directory (in which case any files recursively under that directory will be synced).
      remotePath: container path to which changes will be synced. Must be absolute.
  """
  pass

def run(cmd: Union[str, List[str]], trigger: Union[List[str], str] = []) -> LiveUpdateStep:
  """Specify that the given `cmd` should be executed when updating an image's container

  May not precede any `sync` steps in a `live_update`.

  For more info, see the `Live Update Reference <live_update_reference.html>`_.

  Args:
    cmd: Command to run. If a string, executed with ``sh -c``; if a list, will be passed to the operating system
      as program name and args.

    trigger: If the ``trigger`` argument is specified, the build step is only run when there are changes to the given file(s). Paths relative to Tiltfile. (Note that in addition to matching the trigger, file changes must also match at least one of this Live Update's syncs in order to trigger this run. File changes that do not match any syncs will be ignored.)
  """
  pass

def restart_container() -> LiveUpdateStep:
  """**For use with Docker Compose resources only.**

  Specify that a container should be restarted when it is live-updated. In
  practice, this means that the container re-executes its `ENTRYPOINT` within
  the changed filesystem.

  May only be included in a `live_update` once, and only as the last step.

  For more info (and for the equivalent functionality for Kubernetes resources),
  see the `Live Update Reference <live_update_reference.html#restarting-your-process>`__.
  """
  pass

def docker_build(ref: str,
                 context: str,
                 build_args: Dict[str, str] = {},
                 dockerfile: str = "Dockerfile",
                 dockerfile_contents: Union[str, Blob] = "",
                 live_update: List[LiveUpdateStep]=[],
                 match_in_env_vars: bool = False,
                 ignore: Union[str, List[str]] = [],
                 only: Union[str, List[str]] = [],
                 entrypoint: Union[str, List[str]] = [],
                 target: str = "",
                 ssh: Union[str, List[str]] = "",
                 network: str = "",
                 secret: Union[str, List[str]] = "",
                 extra_tag: Union[str, List[str]] = "",
                 container_args: List[str] = None,
                 cache_from: Union[str, List[str]] = [],
                 pull: bool = False,
                 platform: str = "") -> None:
  """Builds a docker image.

  The invocation

  .. code-block:: python

    docker_build('myregistry/myproj/backend', '/path/to/code')

  is roughly equivalent to the shell call

  .. code-block:: bash

    docker build /path/to/code -t myregistry/myproj/backend

  For more information on the `ignore` and `only` parameters, see our `Guide to File Changes </file_changes.html>`_.

  Note that you can't set both the `dockerfile` and `dockerfile_contents` arguments (will throw an error).

  Note also that the `entrypoint` parameter is not supported for Docker Compose resources.

  When using Docker Compose, Tilt expects the image build to be either managed by your Docker Compose file (via the `build <https://docs.docker.com/compose/compose-file/compose-file-v3/#build>`_ key) OR by Tilt's :meth:`docker_build`, but not both. (Follow this `GitHub issue <https://github.com/tilt-dev/tilt/issues/5196>`_ to be notified of changes to this expectation.)

  Finally, Tilt will put the image in a place where the target runtime can access it. Tilt will make a best effort to detect what kind of runtime you're using (Docker Compose, Kind, GKE, etc), and pick the best strategy for getting the image into it fast. See https://docs.tilt.dev/choosing_clusters.html for more info.

  Args:
    ref: name for this image (e.g. 'myproj/backend' or 'myregistry/myproj/backend'). If this image will be used in a k8s resource(s), this ref must match the ``spec.container.image`` param for that resource(s).
    context: path to use as the Docker build context.
    build_args: build-time variables that are accessed like regular environment variables in the ``RUN`` instruction of the Dockerfile. See `the Docker Build Arg documentation <https://docs.docker.com/engine/reference/commandline/build/#set-build-time-variables---build-arg>`_.
    dockerfile: path to the Dockerfile to build.
    dockerfile_contents: raw contents of the Dockerfile to use for this build.
    live_update: set of steps for updating a running container (see `Live Update documentation <live_update_reference.html>`_).
    match_in_env_vars: specifies that k8s objects can reference this image in their environment variables, and Tilt will handle those variables the same as it usually handles a k8s container spec's ``image`` s.
    ignore: set of file patterns that will be ignored, in addition to ``.git`` directory that's `ignored by default <file_changes.html#where-ignores-come-from>`_. Ignored files will not trigger builds and will not be included in images. Follows the `dockerignore syntax <https://docs.docker.com/engine/reference/builder/#dockerignore-file>`_. Patterns will be evaluated relative to the ``context`` parameter.
    only: set of file paths that should be considered for the build. All other changes will not trigger a build and will not be included in images. Inverse of ignore parameter. Only accepts real paths, not file globs. Patterns will be evaluated relative to the ``context`` parameter.
    entrypoint: command to run when this container starts. Takes precedence over the container's ``CMD`` or ``ENTRYPOINT``, and over a `container command specified in k8s YAML <https://kubernetes.io/docs/tasks/inject-data-application/define-command-argument-container/>`_. If specified as a string, will be evaluated in a shell context (e.g. ``entrypoint="foo.sh bar"`` will be executed in the container as ``/bin/sh -c 'foo.sh bar'``); if specifed as a list, will be passed to the operating system as program name and args.
    target: Specify a build stage in the Dockerfile. Equivalent to the ``docker build --target`` flag.
    ssh: Include SSH secrets in your build. Use ssh='default' to clone private repositories inside a Dockerfile. Uses the syntax in the `docker build --ssh flag <https://docs.docker.com/develop/develop-images/build_enhancements/#using-ssh-to-access-private-data-in-builds>`_.
    network: Set the networking mode for RUN instructions. Equivalent to the ``docker build --network`` flag.
    secret: Include secrets in your build in a way that won't show up in the image. Uses the same syntax as the `docker build --secret flag <https://docs.docker.com/develop/develop-images/build_enhancements/#new-docker-build-secret-information>`_.
    extra_tag: Tag an image with one or more extra references after each build. Useful when running Tilt in a CI pipeline, where you want each image to be tagged with the pipeline ID so you can find it later. Uses the same syntax as the ``docker build --tag`` flag.
    container_args: args to run when this container starts. Takes precedence over a `container args specified in k8s YAML <https://kubernetes.io/docs/tasks/inject-data-application/define-command-argument-container/>`_.
    cache_from: Cache image builds from a remote registry. Uses the same syntax as `docker build --cache-from flag <https://docs.docker.com/engine/reference/commandline/build/#specifying-external-cache-sources>`_.
    pull: Force pull the latest version of parent images. Equivalent to the ``docker build --pull`` flag.
    platform: Target platform for build (e.g. ``linux/amd64``). Defaults to the value of the ``DOCKER_DEFAULT_PLATFORM`` environment variable. Equivalent to the ``docker build --platform`` flag.
  """
  pass

def docker_compose(configPaths: Union[str, Blob, List[Union[str, Blob]]], env_file: str = None, project_name: str = "") -> None:
  """Run containers with Docker Compose.

  Tilt will read your Docker Compose YAML and separate out the services.
  We will infer which services defined in your YAML
  correspond to images defined elsewhere in your ``Tiltfile`` (matching based on
  the DockerImage ref).

  You can set up Docker Compose with a path to a file, a Blob containing Compose YAML, or a list of paths and/or Blobs.

  Tilt will watch your Docker Compose YAML and reload if it changes.

  For more info, see `the guide to Tilt with Docker Compose <docker_compose.html>`_.

  Examples:

  .. code-block:: python

    # Path to file
    docker_compose('./docker-compose.yml')

    # List of files
    docker_compose(['./docker-compose.yml', './docker-compose.override.yml'])

    # Inline compose definition
    services = {'redis': {'image': 'redis', 'ports': '6379:6379'}}
    docker_compose(encode_yaml({'services': services}))

    # File with inline override
    services = {'app': {'environment': {'DEBUG': 'true'}}}
    docker_compose(['docker-compose.yml', encode_yaml({'services': services})])

  Args:
    configPaths: Path(s) and/or Blob(s) to Docker Compose yaml files or content.
    env_file: Path to env file to use; defaults to ``.env`` in current directory.
    project_name: The Docker Compose project name. If unspecified, the main Tiltfile's directory name is used.
  """



def k8s_yaml(yaml: Union[str, List[str], Blob], allow_duplicates: bool = False) -> None:
  """Call this with a path to a file that contains YAML, or with a ``Blob`` of YAML.

  We will infer what (if any) of the k8s resources defined in your YAML
  correspond to Images defined elsewhere in your ``Tiltfile`` (matching based on
  the DockerImage ref and on pod selectors). Any remaining YAML is YAML that Tilt
  applies to your k8s cluster independently.

  Any YAML files are watched (See ``watch_file``).

  Examples:

  .. code-block:: python

    # path to file
    k8s_yaml('foo.yaml')

    # list of paths
    k8s_yaml(['foo.yaml', 'bar.yaml'])

    # Blob, i.e. `local` output (in this case, script output)
    templated_yaml = local('./template_yaml.sh')
    k8s_yaml(templated_yaml)

  Args:
    yaml: Path(s) to YAML, or YAML as a ``Blob``.
    allow_duplicates: If you try to register the same Kubernetes
      resource twice, this function will assume this is a mistake and emit an error.
      Set allow_duplicates=True to allow duplicates. There are some Helm charts
      that have duplicate resources for esoteric reasons.
  """
  pass


def k8s_custom_deploy(name: str,
                      apply_cmd: Union[str, List[str]],
                      delete_cmd: Union[str, List[str]],
                      deps: Union[str, List[str]],
                      image_selector: str="",
                      live_update: List[LiveUpdateStep]=[],
                      apply_dir: str="",
                      apply_env: Dict[str, str]={},
                      apply_cmd_bat: Union[str, List[str]]="",
                      delete_dir: str="",
                      delete_env: Dict[str, str]={},
                      delete_cmd_bat: Union[str, List[str]]="",
                      container_selector: str="",
                      image_deps: List[str]=[]) -> None:
  """Deploy resources to Kubernetes using a custom command.

  For deployment tools that cannot output templated YAML for use with :meth:`k8s_yaml`
  or need to perform additional work as part of deployment, ``k8s_custom_deploy``
  enables integration with Tilt.

  The ``apply_cmd`` will be run whenever a path from ``deps`` changes and should
  output the YAML of the objects it applied to the Kubernetes cluster to stdout.
  Tilt will track workload status and stream pod logs based on this result.

  The ``delete_cmd`` is run on ``tilt down`` so that the tool can clean up any
  objects it created in the cluster as well as any state of its own.

  Both ``apply_cmd`` and ``delete_cmd`` MUST be idempotent. For example, it's
  possible that some objects might already exist when the ``apply_cmd`` is
  invoked, and objects might have already been deleted before ``delete_cmd``
  is invoked. The ``apply_cmd`` should have similar semantics to ``kubectl apply``
  and the ``delete_cmd`` should behave similar to ``kubectl delete --ignore-not-found``.

  Port forwards and other behavior can be configured using :meth:`k8s_resource`
  using the ``name`` as specified here.

  If ``live_update`` rules are specified, exactly one of ``image_selector`` or
  ``container_selector`` must be specified to determine which container(s) are
  eligible for in-place updates. ``image_selector`` will match containers based
  on an image reference, while ``container_selector`` will match a single container
  by name.

  Args:
    name: resource name to use in Tilt UI and for further customization via :meth:`k8s_resource`
    apply_cmd: command that deploys objects to the Kubernetes cluster. If a string, executed with ``sh -c``
      on macOS/Linux, or ``cmd /S /C`` on Windows; if a list, will be passed to the operating system as program name and args.
    delete_cmd: command that deletes objects in the Kubernetes cluster. If a string, executed with ``sh -c``
      on macOS/Linux, or ``cmd /S /C`` on Windows; if a list, will be passed to the operating system as program name and args.
    deps: paths to watch and trigger a re-apply on change
    image_selector: image reference to determine containers eligible for Live Update
    live_update: set of steps for updating a running container (see `Live Update documentation <live_update_reference.html>`_).
    apply_dir: working directory for ``apply_cmd``
    apply_env: environment variables for ``apply_cmd``
    apply_cmd_bat: If non-empty and on Windows, takes precedence over ``apply_cmd``. Ignored on other platforms.
      If a string, executed as a Windows batch command executed with ``cmd /S /C``; if a list, will be passed to
      the operating system as program name and args.
    delete_dir: working directory for ``delete_cmd``
    delete_env: environment variables for ``delete_cmd``
    delete_cmd_bat: If non-empty and on Windows, takes precedence over ``delete_cmd``. Ignored on other platforms.
      If a string, executed as a Windows batch command executed with ``cmd /S /C``; if a list, will be passed to
      the operating system as program name and args.
    container_selector: container name to determine container for Live Update
    image_deps: a list of image builds that this deploy depends on.
      The tagged image names will be injected into the environment of the
      the apply command in the form:

      `TILT_IMAGE_i` - The reference to the image #i (0-based) from the point of view of the cluster container runtime.

      `TILT_IMAGE_MAP_i` - The name of the image map #i (0-based) with the current status of the image.
  """
  pass


class TriggerMode:
  """A set of constants that describe how Tilt triggers an update for a resource.
  Possible values are:

  - ``TRIGGER_MODE_AUTO``: the default. When Tilt detects a change to files or config files associated with this resource, it triggers an update.

  - ``TRIGGER_MODE_MANUAL``: user manually triggers update for dirty resources (i.e. resources with pending changes) via a button in the UI. (Note that the initial build always occurs automatically.)

  The default trigger mode for all manifests may be set with the top-level function :meth:`trigger_mode`
  (if not set, defaults to ``TRIGGER_MODE_AUTO``), and per-resource with :meth:`k8s_resource` / :meth:`dc_resource`.

  See also: `Manual Update Control documentation <manual_update_control.html>`_
  """
  def __init__(self):
    pass

def trigger_mode(trigger_mode: TriggerMode):
  """Sets the default :class:`TriggerMode` for resources in this Tiltfile.
  (Trigger mode may still be adjusted per-resource with :meth:`k8s_resource`.)

  If this function is not invoked, the default trigger mode for all resources is ``TRIGGER MODE AUTO``.

  See also: `Manual Update Control documentation <manual_update_control.html>`_

  Args:
    trigger_mode: may be one of ``TRIGGER_MODE_AUTO`` or ``TRIGGER_MODE_MANUAL``

  """

# Hack so this appears correctly in the function signature: https://stackoverflow.com/a/50193319/4628866
TRIGGER_MODE_AUTO = type('_sentinel', (TriggerMode,),
                 {'__repr__': lambda self: 'TRIGGER_MODE_AUTO'})()

def dc_resource(name: str,
                trigger_mode: TriggerMode = TRIGGER_MODE_AUTO,
                resource_deps: List[str] = [],
                links: Union[str, Link, List[Union[str, Link]]] = [],
                labels: Union[str, List[str]] = [],
                auto_init: bool = True) -> None:
  """Configures the Docker Compose resource of the given name. Note: Tilt does an amount of resource configuration
  for you(for more info, see `Tiltfile Concepts: Resources <tiltfile_concepts.html#resources>`_); you only need
  to invoke this function if you want to configure your resource beyond what Tilt does automatically.

  Args:
    name: The name of the resource in the docker-compose yaml.
    trigger_mode: one of ``TRIGGER_MODE_AUTO`` or ``TRIGGER_MODE_MANUAL``. For more info, see the
      `Manual Update Control docs <manual_update_control.html>`_.
    resource_deps: a list of resources on which this resource depends.
      See the `Resource Dependencies docs <resource_dependencies.html>`_.
    links: one or more links to be associated with this resource in the UI. For more info, see
      `Accessing Resource Endpoints <accessing_resource_endpoints.html#arbitrary-links>`_.
    labels: used to group resources in the Web UI, (e.g. you want all frontend services displayed together, while test and backend services are displayed seperately). A label must start and end with an alphanumeric character, can include ``_``, ``-``, and ``.``, and must be 63 characters or less. For an example, see `Resource Grouping <tiltfile_concepts.html#resource-groups>`_.
    auto_init: whether this resource runs on ``tilt up``. Defaults to ``True``. For more info, see the
      `Manual Update Control docs <manual_update_control.html>`_.
  """

  pass

def k8s_resource(workload: str = "", new_name: str = "",
                 port_forwards: Union[str, int, PortForward, List[Union[str, int, PortForward]]] = [],
                 extra_pod_selectors: Union[Dict[str, str], List[Dict[str, str]]] = [],
                 trigger_mode: TriggerMode = TRIGGER_MODE_AUTO,
                 resource_deps: List[str] = [], objects: List[str] = [],
                 auto_init: bool = True,
                 pod_readiness: str = "",
                 links: Union[str, Link, List[Union[str, Link]]]=[],
                 labels: Union[str, List[str]] = [],
                 discovery_strategy: str = "") -> None:
  """

  Configures or creates the specified Kubernetes resource.

  A "resource" is a bundle of work managed by Tilt: a Kubernetes resource consists
  of one or more Kubernetes objects to deploy, and zero or more image build directives
  for the images referenced therein.

  Tilt assembles Kubernetes resources automatically, as described in
  `Tiltfile Concepts: Resources <tiltfile_concepts.html#resources>`_. You may call
  ``k8s_resource`` to configure an automatically created Kubernetes resource, or to
  create and configure a new one:

  - If configuring an automatically created resource: the ``workload`` parameter must be specified.
  - If creating a new resource: both the ``objects`` and ``new_name`` parameters must be specified.

  Calling ``k8s_resource`` is *optional*; you can use this function to configure port forwarding for
  your resource, to rename it, or to adjust any of the other settings specified below, but in many cases,
  Tilt's default behavior is sufficient.

  Examples:

  .. code-block:: python

    # load Deployment foo
    k8s_yaml('foo.yaml')

    # modify the resource called "foo" (auto-assembled by Tilt)
    # to forward container port 8080 to localhost:8080
    k8s_resource(workload='foo', port_forwards=8080)

  .. code-block:: python

    # load CRD "bar", Service "bar", and Secret "bar-password"
    k8s_yaml('bar.yaml')

    # create a new resource called "bar" which contains the objects
    # loaded above (none of which are workloads, so none of which
    # would be automatically assigned to a resource). Note that the
    # first two object selectors specify both 'name' and 'kind',
    # since just the string "bar" does not uniquely specify a single object.
    # As the object name "bar-password" is unique, "bar-password" suffices as
    # an object selector (though a more more qualified object selector
    # like "bar-password:secret" or "bar-password:secret:default" would
    # be accepted as well).
    k8s_resource(
      objects=['bar:crd', 'bar:service', 'bar-password'],
      new_name='bar'
    )

  For more examples, see `Tiltfile Concepts: Resources <tiltfile_concepts.html#resources>`_.

  Args:
    workload: The name of an existing auto-assembled resource to configure
      (Tilt generates resource names when it `assembles resources by workload <tiltfile_concepts.html#resources>`_).
      (If you instead want to create/configure a _new_ resource, use the ``objects`` parameter
      in conjunction with ``new_name``.)
    new_name: If non-empty, will be used as the new name for this resource. (To
      programmatically rename all resources, see :meth:`workload_to_resource_function`.)
    port_forwards: Host port to connect to the pod. Takes 3 forms:

      ``'9000'`` (port only) - Connect localhost:9000 to the container's port 9000,
      if it is exposed. Otherwise connect to the container's default port.

      ``'9000:8000'`` (host port to container port) - Connect localhost:9000 to the container port 8000).

      ``'elastic.local:9200:8000'`` (host address to container port) - Bind elasticsearch:9200 on the host
      to container port 8000. You will also need to update /etc/host to make 'elastic.local' point to localhost.

      Multiple port forwards can be specified (e.g., ``['9000:8000', '9001:8001']``).
      The string-based syntax is sugar over the more explicit ``port_forward(9000, 8000)``.
    extra_pod_selectors: In addition to relying on Tilt's heuristics to automatically
      find Kubernetes resources associated with this resource, a user may specify extra
      labelsets to force pods to be associated with this resource. An pod
      will be associated with this resource if it has all of the labels in at
      least one of the entries specified (but still also if it meets any of
      Tilt's usual mechanisms).
    trigger_mode: One of ``TRIGGER_MODE_AUTO`` or ``TRIGGER_MODE_MANUAL``. For more info, see the
      `Manual Update Control docs <manual_update_control.html>`_.
    resource_deps: A list of resources on which this resource depends.
      See the `Resource Dependencies docs <resource_dependencies.html>`_.
    objects: A list of Kubernetes objects to be added to this resource, specified via
      Tilt's `Kubernetes Object Selector <tiltfile_concepts.html#kubernetes-object-selectors>`_
      syntax. If the ``workload`` parameter is specified, these objects will be
      added to the existing resource; otherwise, these objects will form a new
      resource with name ``new_name``. If an object selector matches more than
      one Kubernetes object, or matches an object already associated with a
      resource, ``k8s_resource`` raises an error.
    auto_init: whether this resource runs on ``tilt up``. Defaults to ``True``. For more info, see the
      `Manual Update Control docs <manual_update_control.html>`_.
    pod_readiness: Possible values: 'ignore', 'wait'. Controls whether Tilt waits for
      pods to be ready before the resource is considered healthy (and dependencies
      can start building). By default, Tilt will wait for pods to be ready if it
      thinks a resource has pods.
    links: one or more links to be associated with this resource in the UI. For more info, see
      `Accessing Resource Endpoints <accessing_resource_endpoints.html#arbitrary-links>`_.
    labels: used to group resources in the Web UI, (e.g. you want all frontend services displayed together, while test and backend services are displayed seperately). A label must start and end with an alphanumeric character, can include ``_``, ``-``, and ``.``, and must be 63 characters or less. For an example, see `Resource Grouping <tiltfile_concepts.html#resource-groups>`_.
    discovery_strategy: Possible values: '', 'default', 'selectors-only'. When '' or 'default', Tilt both uses `extra_pod_selectors` and traces k8s owner references to identify this resource's pods. When 'selectors-only', Tilt uses only `extra_pod_selectors`.
  """
  pass

def filter_yaml(yaml: Union[str, List[str], Blob], labels: dict=None, name: str=None, namespace: str=None, kind: str=None, api_version: str=None):
  """Call this with a path to a file that contains YAML, or with a ``Blob`` of YAML.
  (E.g. it can be called on the output of ``kustomize`` or ``helm``.)

  Captures the YAML entities that meet the filter criteria and returns them as a blob;
  returns the non-matching YAML as the second return value.

  For example, if you have a file of *all* your YAML, but only want to pass a few elements to Tilt: ::

    # extract all YAMLs matching labels "app=foobar"
    foobar_yaml, rest = filter_yaml('all.yaml', labels={'app': 'foobar'})
    k8s_yaml(foobar_yaml)

    # extract YAMLs of kind "deployment" with metadata.name regex-matching "baz", also matching "bazzoo" and "bar-baz"
    baz_yaml, rest = filter_yaml(rest, name='baz', kind='deployment')
    k8s_yaml(baz_yaml)

    # extract YAMLs of kind "deployment" exactly matching metadata.name "foo"
    foo_yaml, rest = filter_yaml(rest, name='^foo$', kind='deployment')
    k8s_yaml(foo_yaml)

  Args:
    yaml: Path(s) to YAML, or YAML as a ``Blob``.
    labels: return only entities matching these labels. (Matching entities
      must satisfy all of the specified label constraints, though they may have additional
      labels as well: see the `Kubernetes docs <https://kubernetes.io/docs/concepts/overview/working-with-objects/labels/>`_
      for more info.)
    name: Case-insensitive regexp specifying the ``metadata.name`` property of entities to match
    namespace: Case-insensitive regexp specifying the ``metadata.namespace`` property of entities to match
    kind: Case-insensitive regexp specifying the kind of entities to match (e.g. "Service", "Deployment", etc.).
    api_version: Case-insensitive regexp specifying the apiVersion for `kind`, (e.g., "apps/v1")

  Returns:
    2-element tuple containing

    - **matching** (:class:`~api.Blob`): blob of YAML entities matching given filters
    - **rest** (:class:`~api.Blob`): the rest of the YAML entities
  """
  pass

def include(path: str):
  """Execute another Tiltfile.

  Discouraged. Please use :meth:`load` or :meth:`load_dynamic`.

  Example ::

    include('./frontend/Tiltfile')
    include('./backend/Tiltfile')
  """

def load(path: str, *args):
  """Execute another Tiltfile, and import the named variables into the current scope.

  Used when you want to define common functions or constants
  to share across Tiltfiles.

  Example ::

    load('./lib/Tiltfile', 'create_namespace')
    create_namespace('frontend')

  A Tiltfile may only be executed once. If a Tiltfile is loaded multiple times,
  the second load will use the results of the last execution.

  If ``path`` starts with ``"ext://"`` the path will be treated as a `Tilt Extension <extensions.html>`_.

  Example ::

    load('ext://hello_world', 'hi') # Resolves to https://github.com/tilt-dev/tilt-extensions/blob/master/hello_world/Tiltfile
    hi() # prints "Hello world!"

  Note that ``load()`` is a language built-in. Read the
  `specification <https://github.com/google/starlark-go/blob/master/doc/spec.md#load-statements>`_
  for its complete syntax.

  Because ``load()`` is analyzed at compile-time, the first argument MUST be a string literal.
  """

def load_dynamic(path: str) -> Dict[str, Any]:
  """Execute another Tiltfile, and return a dict of the global variables it creates.

  Used when you want to define common functions or constants
  to share across Tiltfiles.

  Example ::

    symbols = load_dynamic('./lib/Tiltfile')
    create_namespace = symbols['create_namespace']
    create_namespace('frontend')

  Like :meth:`load`, each Tiltfile will only be executed once. Can also be used to load a `Tilt Extension <extensions.html>`_.

  Because ``load_dynamic()`` is executed at run-time, you can use it to do
  meta-programming that you cannot do with ``load()`` (like determine which file
  to load by running a script first). But you need to unpack the variables yourself -
  you don't get the nice syntactic sugar of binding local variables.
  """

def local(command: Union[str, List[str]],
          quiet: bool = False,
          command_bat: Union[str, List[str]] = "",
          echo_off: bool = False,
          env: Dict[str, str] = {},
          dir: str = "",
          stdin: Union[str, Blob, None] = None) -> Blob:
  """Runs a command on the *host* machine, waits for it to finish, and returns its stdout as a ``Blob``

  Args:
    command: Command to run. If a string, executed with ``sh -c`` on macOS/Linux, or ``cmd /S /C`` on Windows;
      if a list, will be passed to the operating system as program name and args.
    quiet: If set to True, skips printing output to log.
    command_bat: If non-empty and on Windows, takes precedence over ``command``. Ignored on other platforms.
      If a string, executed as a Windows batch command executed with ``cmd /S /C``; if a list, will be passed to
      the operating system as program name and args.
    echo_off: If set to True, skips printing command to log.
    env: Environment variables to pass to the executed ``command``. Values specified here will override any variables passed to the Tilt parent process.
    dir: Working directory for ``command``. Defaults to the Tiltfile's location.
    stdin: If not ``None``, will be written to ``command``'s stdin.
  """
  pass

def read_file(file_path: str, default: str = None) -> Blob:
  """
  Reads file and returns its contents.

  If the `file_path` does not exist and `default` is not `None`, `default` will be returned.
  In any other case, an error reading `file_path` will be a Tiltfile load error.

  Args:
    file_path: Path to the file locally (absolute, or relative to the location of the Tiltfile).
    default: If not `None` and the file at `file_path` does not exist, this value will be returned."""
  pass

def watch_file(file_path: str) -> None:
  """Watches a file. If the file is changed a re-exectution of the Tiltfile is triggered.

  If the path is a directory, its contents will be recursively watched.

  Args:
    file_path: Path to the file locally (absolute, or relative to the location of the Tiltfile)."""


def kustomize(pathToDir: str, kustomize_bin: str = None, flags: List[str] = []) -> Blob:
  """Run `kustomize <https://github.com/kubernetes-sigs/kustomize>`_ on a given directory and return the resulting YAML as a Blob
  Directory is watched (see ``watch_file``). Checks for and uses separately installed kustomize first, if it exists. Otherwise,
  uses kubectl's kustomize. See `blog post <https://blog.tilt.dev/2020/02/04/are-you-my-kustomize.html>`_.

  Args:
    pathToDir: Path to the directory locally (absolute, or relative to the location of the Tiltfile).
    kustomize_bin: Custom path to the ``kustomize`` binary executable. Defaults to searching $PATH for kustomize.
    flags: Additional flags to pass to ``kustomize build``
  """
  pass

def helm(pathToChartDir: str, name: str = "", namespace: str = "", values: Union[str, List[str]]=[], set: Union[str, List[str]]=[], kube_version: str = "") -> Blob:
  """Run `helm template <https://docs.helm.sh/helm/#helm-template>`_ on a given directory that contains a chart and return the fully rendered YAML as a Blob
  Chart directory is watched (See ``watch_file``).

  For more examples, see the `Helm Cookbook <helm.html>`_.

  Args:
    pathToChartDir: Path to the directory locally (absolute, or relative to the location of the Tiltfile).
    name: The release name. Equivalent to the helm `--name` flag
    namespace: The namespace to deploy the chart to. Equivalent to the helm `--namespace` flag
    values: Specify one or more values files (in addition to the `values.yaml` file in the chart). Equivalent to the Helm ``--values`` or ``-f`` flags (`see docs <https://helm.sh/docs/chart_template_guide/#values-files>`_).
    set: Specify one or more values. Equivalent to the Helm ``--set`` flag.
    kube_version: Specify for which kubernetes version template will be generated. Equivalent to the Helm ``--kube-version`` flag.
"""
  pass

def blob(contents: str) -> Blob:
  """Creates a Blob object that wraps the provided string. Useful for passing strings in to functions that expect a `Blob`, e.g. ``k8s_yaml``."""
  pass

def listdir(directory: str, recursive: bool = False) -> List[str]:
  """Returns all the files of the provided directory.

  If ``recursive`` is set to ``True``, the directory's contents will be recursively watched, and a change to any file will trigger a re-execution of the Tiltfile.

  This function returns absolute paths. Subdirectory names are not returned.

  Args:
    directory: Path to the directory locally (absolute, or relative to the location of the Tiltfile).
    recursive: Walk the given directory tree recursively and return all files in it; additionally, recursively watch for changes in the directory tree.
  """
  pass

def k8s_kind(kind: str, api_version: str=None, *, image_json_path: Union[str, List[str]]=[], image_object_json_path: Dict=None, pod_readiness: str=""):
  """Tells Tilt about a k8s kind.

  For CRDs that use images built by Tilt: call this with `image_json_path` or
  `image_object` to tell Tilt where in the CRD's spec the image is specified.

  For CRDs that do not use images built by Tilt, but have pods you want in a Tilt resource: call this without `image_json_path`, simply to specify that this type is a Tilt workload. Then call :meth:`k8s_resource` with `extra_pod_selectors` to specify which pods Tilt should associate with this resource.

  (Note the `*` in the signature means `image_json_path` must be passed as a keyword, e.g., `image_json_path="{.spec.image}"`)

  Example ::

    # Fission has a CRD named "Environment"
    k8s_yaml('deploy/fission.yaml')
    k8s_kind('Environment', image_json_path='{.spec.runtime.image}')

  Here's an example that specifies the image location in `a UselessMachine
  Custom Resource
  <https://github.com/tilt-dev/tilt/blob/master/integration/crd/Tiltfile#L8>`_.

  Args:
    kind: Case-insensitive regexp specifying he value of the `kind` field in the k8s object definition (e.g., `"Deployment"`)
    api_version: Case-insensitive regexp specifying the apiVersion for `kind`, (e.g., "apps/v1")
    image_json_path: Either a string or a list of string containing json path(s) within that kind's definition
      specifying images deployed with k8s objects of that type.
      This uses the k8s json path template syntax, described `here <https://kubernetes.io/docs/reference/kubectl/jsonpath/>`_.
    image_object: A specifier of the form `image_object={'json_path': '{.path.to.field}', 'repo_field': 'repo', 'tag_field': 'tag'}`.
      Used to tell Tilt how to inject images into Custom Resources that express the image repo and tag as separate fields.
    pod_readiness: Possible values: 'ignore', 'wait'. Controls whether Tilt waits for
      pods to be ready before the resource is considered healthy (and dependencies
      can start building). By default, Tilt will wait for pods to be ready if it
      thinks a resource has pods. This can be overridden on a resource-by-resource basis
      by the `k8s_resource` function.

  """
  pass

StructuredDataType = Union[
    Dict[str, Any],
    List[Any],
]

def decode_json(json: Union[str, Blob]) -> StructuredDataType:
  """
  Deserializes the given JSON into a starlark object

  Args:
    json: the JSON to deserialize
  """
  pass

def encode_json(obj: StructuredDataType) -> Blob:
  """
  Serializes the given starlark object into JSON.

  Only supports maps with string keys, lists, strings, ints, and bools.

  Args:
    obj: the object to serialize
  """
  pass

def read_json(path: str, default: str = None) -> StructuredDataType:
  """
  Reads the file at `path` and deserializes its contents as JSON

  If the `path` does not exist and `default` is not `None`, `default` will be returned.
  In any other case, an error reading `path` will be a Tiltfile load error.

  Args:
    path: Path to the file locally (absolute, or relative to the location of the Tiltfile).
    default: If not `None` and the file at `path` does not exist, this value will be returned."""
  pass

def read_yaml(path: str, default: StructuredDataType = None) -> StructuredDataType:
  """
  Reads the file at `path` and deserializes its contents into a starlark object

  If the `path` does not exist and `default` is not `None`, `default` will be returned.
  In any other case, an error reading `path` will be a Tiltfile load error.

  Args:
    path: Path to the file locally (absolute, or relative to the location of the Tiltfile).
    default: If not `None` and the file at `path` does not exist, this value will be returned."""
  pass

def read_yaml_stream(path: str, default: List[StructuredDataType] = None) -> List[StructuredDataType]:
  """
  Reads a yaml stream (i.e., yaml documents separated by ``"\\n---\\n"``) from the
  file at `path` and deserializes its contents into a starlark object

  If the `path` does not exist and `default` is not `None`, `default` will be returned.
  In any other case, an error reading `path` will be a Tiltfile load error.

  Args:
    path: Path to the file locally (absolute, or relative to the location of the Tiltfile).
    default: If not `None` and the file at `path` does not exist, this value will be returned."""
  pass

def decode_yaml(yaml: Union[str, Blob]) -> StructuredDataType:
  """
  Deserializes the given yaml document into a starlark object

  Args:
    yaml: the yaml to deserialize
  """
  pass

def decode_yaml_stream(yaml: Union[str, Blob]) -> List[StructuredDataType]:
  """
  Deserializes the given yaml stream (i.e., any number of yaml
  documents, separated by ``"\\n---\\n"``) into a list of starlark objects.

  Args:
    yaml: the yaml to deserialize
  """
  pass

def encode_yaml(obj: StructuredDataType) -> Blob:
  """
  Serializes the given starlark object into YAML.

  Only supports maps with string keys, lists, strings, ints, and bools.

  Args:
    obj: the object to serialize
  """
  pass

def encode_yaml_stream(objs: List[StructuredDataType]) -> Blob:
  """
  Serializes the given starlark objects into a YAML stream (i.e.,
  multiple YAML documents, separated by ``"\\n---\\n"``).

  Only supports maps with string keys, lists, strings, ints, and bools.

  Args:
    objs: the object to serialize
  """
  pass

def default_registry(host: str, host_from_cluster: str = None, single_name: str = "") -> None:
  """Specifies that any images that Tilt builds should be renamed so that they have the specified Docker registry.

  This is useful if, e.g., a repo is configured to push to Google Container Registry, but you want to use Elastic Container Registry instead, without having to edit a bunch of configs. For example, ``default_registry("gcr.io/myrepo")`` would cause ``docker.io/alpine`` to be rewritten to ``gcr.io/myrepo/docker.io_alpine``

  For more info, see our `Using a Personal Registry Guide <personal_registry.html>`_.

  Args:
    host: host of the registry that all built images should be renamed to use.
    host_from_cluster: registry host to use when referencing images from inside the cluster (i.e. in Kubernetes YAML). Only include this arg if it is different from ``host``. For more on this use case, `see this guide <personal_registry.html#different-urls-from-inside-your-cluster>`_.
    single_name: In ECR, each repository in a registry needs to be created up-front. single_name lets you
      set a single repository to push to (e.g., a personal dev repository), and embeds the image name in the
      tag instead.

  Images are renamed following these rules:

  1. Replace ``/`` and ``@`` with ``_``.

  2. Prepend the value of ``host`` and a ``/``.

  e.g., with ``default_registry('gcr.io/myorg')``, an image called ``user-service`` becomes ``gcr.io/myorg/user-service``.
  """
  pass

def custom_build(
    ref: str,
    command: Union[str, List[str]],
    deps: List[str],
    tag: str = "",
    disable_push: bool = False,
    skips_local_docker: bool = False,
    live_update: List[LiveUpdateStep]=[],
    match_in_env_vars: bool = False,
    ignore: Union[str, List[str]] = [],
    entrypoint: Union[str, List[str]] = [],
    command_bat_val: str = "",
    outputs_image_ref_to: str = "",
    command_bat: Union[str, List[str]] = "",
    image_deps: List[str] = []):
  """Provide a custom command that will build an image.

  Example ::

    custom_build(
      'gcr.io/my-project/frontend-server',
      'docker build -t $EXPECTED_REF .',
      ['.'],
    )

  Please read the `Custom Image Builders Guide <custom_build.html>`_ on how to
  use this function.

  All custom build scripts build an image and put it somewhere. But there are
  several different patterns for where they put the image, how they compute a
  digest of the contents, and how they push the image to the
  cluster. ``custom_build`` has many options to support different combinations
  of each mode. The guide has some examples of common combinations.

  Args:
    ref: name for this image (e.g. 'myproj/backend' or 'myregistry/myproj/backend'). If this image will be used in a k8s resource(s), this ref must match the ``spec.container.image`` param for that resource(s).
    command: a command that, when run in the shell, builds an image puts it in the registry as ``ref``. In the
      default mode, must produce an image named ``$EXPECTED_REF``.  If a string, executed with ``sh -c`` on macOS/Linux,
      or ``cmd /S /C`` on Windows; if a list, will be passed to the operating system as program name and args.
    deps: a list of files or directories to be added as dependencies to this image. Tilt will watch those files and will rebuild the image when they change. Only accepts real paths, not file globs.
    tag: Some tools can't change the image tag at runtime. They need a pre-specified tag. Tilt will set ``$EXPECTED_REF = image_name:tag``,
       then re-tag it with its own tag before pushing to your cluster.
    disable_push: whether Tilt should push the image in to the registry that the Kubernetes cluster has access to. Set this to true if your command handles pushing as well.
    skips_local_docker: Whether your build command writes the image to your local Docker image store. Set this to true if you're using a cloud-based builder or independent image builder like ``buildah``.
    live_update: set of steps for updating a running container (see `Live Update documentation <live_update_reference.html>`_).
    match_in_env_vars: specifies that k8s objects can reference this image in their environment variables, and Tilt will handle those variables the same as it usually handles a k8s container spec's ``image`` s.
    ignore: set of file patterns that will be ignored. Ignored files will not trigger builds and will not be included in images. Follows the `dockerignore syntax <https://docs.docker.com/engine/reference/builder/#dockerignore-file>`_. Patterns/filepaths will be evaluated relative to each ``dep`` (e.g. if you specify ``deps=['dep1', 'dep2']`` and ``ignores=['foobar']``, Tilt will ignore both ``deps1/foobar`` and ``dep2/foobar``).
    entrypoint: command to run when this container starts. Takes precedence over the container's ``CMD`` or ``ENTRYPOINT``, and over a `container command specified in k8s YAML <https://kubernetes.io/docs/tasks/inject-data-application/define-command-argument-container/>`_. If specified as a string, will be evaluated in a shell context (e.g. ``entrypoint="foo.sh bar"`` will be executed in the container as ``/bin/sh -c 'foo.sh bar'``); if specifed as a list, will be passed to the operating system as program name and args. Kubernetes-only.
    command_bat_val: Deprecated, use command_bat.
    outputs_image_ref_to: Specifies a file path. When set, the custom build command must write a content-based
      tagged image ref to this file. Tilt will read that file after the cmd runs to get the image ref,
      and inject that image ref into the YAML. For more on content-based tags, see <custom_build.html#why-tilt-uses-immutable-tags>_
    command_bat: If non-empty and on Windows, takes precedence over ``command``. Ignored on other platforms.
      If a string, executed as a Windows batch command executed with ``cmd /S /C``; if a list, will be passed to
      the operating system as program name and args.
    image_deps: a list of image builds that this deploy depends on.
      The tagged image names will be injected into the environment of the
      the custom build command in the form:

      `TILT_IMAGE_i` - The reference to the image #i (0-based) from the point of view of the local host.

      `TILT_IMAGE_MAP_i` - The name of the image map #i (0-based) with the current status of the image.

  """
  pass


class K8sObjectID:
  """
  Attributes:
    name (str): The object's name (e.g., `"my-service"`)
    kind (str): The object's kind (e.g., `"deployment"`)
    namespace (str): The object's namespace (e.g., `"default"`)
    group (str): The object's group (e.g., `"apps"`)
  """
  pass


def workload_to_resource_function(fn: Callable[[K8sObjectID], str]) -> None:
    """
    Provide a function that will be used to name `Tilt resources <tiltfile_concepts.html#resources>`_.

    Tilt will auto-generate resource names for you. If you do not like the names
    it generates, you can use this to customize how Tilt generates names.

    Example ::

      # name all tilt resources after the k8s object namespace + name
      def resource_name(id):
        return id.namespace + '-' + id.name
      workload_to_resource_function(resource_name)

    The names it generates must be unique (i.e., two workloads can't map to the
    same resource name).

    Args:
      fn: A function that takes a :class:`K8sObjectID` and returns a `str`.
          Tilt will call this function once for each workload to determine that workload's resource's name.
    """

    pass

def k8s_context() -> str:
  """Returns the name of the Kubernetes context Tilt is connecting to.

  Example ::

    if k8s_context() == 'prod':
      fail("failing early to avoid overwriting prod")
  """
  pass

def k8s_namespace() -> str:
  """Returns the name of the Kubernetes namespace Tilt is connecting to.

  Example ::

    if k8s_namespace() == 'default':
      fail("failing early to avoid deploying to 'default' namespace")
  """
  pass

def allow_k8s_contexts(contexts: Union[str, List[str]]) -> None:
  """Specifies that Tilt is allowed to run against the specified k8s context names.

  To help reduce the chances you accidentally use Tilt to deploy to your
  production cluster, Tilt will only push to clusters that have been allowed
  for local development.

  By default, Tilt automatically allows Minikube, Docker for Desktop, Microk8s,
  Red Hat CodeReady Containers, Kind, K3D, and Krucible.

  To add your development cluster to the allow list, add a line in your Tiltfile::

    allow_k8s_contexts('context-name')

  where 'context-name' is the name returned by `kubectl config current-context`.

  If your team connects to many remote dev clusters, a common approach is to
  disable the check entirely and add your own validation::

    allow_k8s_contexts(k8s_context())
    local('./validate-dev-cluster.sh')

  For more on which cluster context is right for you, see `Choosing a Local Dev Cluster <choosing_clusters.html>`_.

  Args:
    contexts: a string or list of strings, specifying one or more k8s context
        names that Tilt is allowed to run in. This list is in addition to
        the default of all known-local clusters.

  Example ::

    allow_k8s_contexts('my-staging-cluster')

    allow_k8s_contexts([
      'my-staging-cluster',
      'gke_my-project-name_my-dev-cluster-name'
    ])

    allow_k8s_contexts(k8s_context()) # disable check

  """
  pass

def enable_feature(feature_name: str) -> None:
  """Configures Tilt to enable non-default features (e.g., experimental or deprecated).

  The Tilt features controlled by this are generally in an unfinished state, and
  not yet documented.

  As a Tiltfile author, you don't need to worry about this function unless something
  else directs you to (e.g., an experimental feature doc, or a conversation with a
  Tilt contributor).

  As a Tiltfile reader, you can probably ignore this, or you can ask the person
  who added it to the Tiltfile what it's doing there.

  Args:
    feature_name: name of the feature to enable
  """
  pass

def local_resource(name: str,
                   cmd: Union[str, List[str]],
                   deps: Union[str, List[str]] = None,
                   trigger_mode: TriggerMode = TRIGGER_MODE_AUTO,
                   resource_deps: List[str] = [],
                   ignore: Union[str, List[str]] = [],
                   auto_init: bool=True,
                   serve_cmd: Union[str, List[str]] = "",
                   cmd_bat: Union[str, List[str]] = "",
                   serve_cmd_bat: Union[str, List[str]] = "",
                   allow_parallel: bool=False,
                   links: Union[str, Link, List[Union[str, Link]]]=[],
                   tags: List[str] = [],
                   env: Dict[str, str] = {},
                   serve_env: Dict[str, str] = {},
                   readiness_probe: Probe = None,
                   dir: str = "",
                   serve_dir: str = "",
                   labels: List[str] = []) -> None:
  """Configures one or more commands to run on the *host* machine (not in a remote cluster).

  By default, Tilt performs an update on local resources on ``tilt up`` and whenever any of their ``deps`` change.

  When Tilt performs an update on a local resource:

  - if `cmd` is non-empty, it is executed
  - if `cmd` succeeds:
    - Tilt kills any extant `serve_cmd` process from previous updates of this resource
    - if `serve_cmd` is non-empty, it is executed

  For more info, see the `Local Resource docs <local_resource.html>`_.

  Args:
    name: will be used as the new name for this resource
    cmd: command to be executed on host machine.  If a string, executed with ``sh -c`` on macOS/Linux, or ``cmd /S /C`` on Windows; if a list, will be passed to the operating system as program name and args.
    deps: a list of files or directories to be added as dependencies to this cmd. Tilt will watch those files and will run the cmd when they change. Only accepts real paths, not file globs.
    trigger_mode: one of ``TRIGGER_MODE_AUTO`` or ``TRIGGER_MODE_MANUAL``. For more info, see the
      `Manual Update Control docs <manual_update_control.html>`_.
    resource_deps: a list of resources on which this resource depends.
      See the `Resource Dependencies docs <resource_dependencies.html>`_.
    ignore: set of file patterns that will be ignored. Ignored files will not trigger runs. Follows the `dockerignore syntax <https://docs.docker.com/engine/reference/builder/#dockerignore-file>`_. Patterns will be evaluated relative to the Tiltfile.
    auto_init: whether this resource runs on ``tilt up``. Defaults to ``True``. For more info, see the
      `Manual Update Control docs <manual_update_control.html>`_.
    serve_cmd: Tilt will run this command on update and expect it to not exit. If a string, executed with
      ``sh -c`` on macOS/Linux, or ``cmd /S /C`` on Windows; if a list, will be passed to the operating
      system as program name and args.
    cmd_bat: If non-empty and on Windows, takes precedence over ``cmd``. Ignored on other platforms.
      If a string, executed as a Windows batch command executed with ``cmd /S /C``; if a list, will be passed to
      the operating system as program name and args.
    serve_cmd_bat: If non-empty and on Windows, takes precedence over ``serve_cmd``. Ignored on other platforms.
      If a string, executed as a Windows batch command executed with ``cmd /S /C``; if a list, will be passed to
      the operating system as program name and args.
    allow_parallel: By default, all local resources are presumed unsafe to run in parallel, due to race
      conditions around modifying a shared file system. Set to True to allow them to run in parallel.
    links: one or more links to be associated with this resource in the Web UI (e.g. perhaps you have a "reset database" workflow and want to attach a link to the database web console). Provide one or more strings (the URLs to link to) or :class:`~api.Link` objects.
    env: Environment variables to pass to the executed ``cmd``. Values specified here will override any variables passed to the Tilt parent process.
    serve_env: Environment variables to pass to the executed ``serve_cmd``. Values specified here will override any variables passed to the Tilt parent process.
    readiness_probe: Optional readiness probe to use for determining ``serve_cmd`` health state. Fore more info, see the :meth:`probe` function.
    dir: Working directory for ``cmd``. Defaults to the Tiltfile directory.
    serve_dir: Working directory for ``serve_cmd``. Defaults to the Tiltfile directory.
    labels: used to group resources in the Web UI, (e.g. you want all frontend services displayed together, while test and backend services are displayed seperately). A label must start and end with an alphanumeric character, can include ``_``, ``-``, and ``.``, and must be 63 characters or less. For an example, see `Resource Grouping <tiltfile_concepts.html#resource-groups>`_.
  """
  pass

def disable_snapshots() -> None:
    """Disables Tilt's `snapshots <snapshots.html>`_ feature, hiding it from the UI.

    This is intended for use in projects where there might be some kind of
    data policy that does not allow developers to upload snapshots to TiltCloud.

    Note that this directive does not provide any real security, since a
    developer can always simply edit it out of the Tiltfile, but it at least
    ensures a pretty high bar of intent.
    """

def docker_prune_settings(disable: bool=False, max_age_mins: int=360,
                          num_builds: int=0, interval_hrs: int=1, keep_recent: int=2) -> None:
  """
  Configures Tilt's Docker Pruner, which runs occasionally in the background and prunes Docker images associated
  with your current project.

  The pruner runs soon after startup (as soon as at least some resources are declared, and there are no pending builds).
  Subsequently, it runs after every ``num_builds`` Docker builds, or, if ``num_builds`` is not set, every ``interval_hrs`` hours.

  The pruner will prune:
    - stopped containers built by Tilt that are at least ``max_age_mins`` mins old
    - images built by Tilt and associated with this Tilt run that are at least ``max_age_mins`` mins old,
      and not in the ``keep_recent`` most recent builds for that image name
    - dangling build caches that are at least ``max_age_mins`` mins old

  Args:
    disable: if true, disable the Docker Pruner
    max_age_mins: maximum age, in minutes, of images/containers to retain. Defaults to 360 mins., i.e. 6 hours
    num_builds: number of Docker builds after which to run a prune. (If unset, the pruner instead runs every ``interval_hrs`` hours)
    interval_hrs: run a Docker Prune every ``interval_hrs`` hours (unless ``num_builds`` is set, in which case use the "prune every X builds" logic). Defaults to 1 hour
    keep_recent: when pruning, retain at least the ``keep_recent`` most recent images for each image name. Defaults to 2
  """
  pass

def analytics_settings(enable: bool) -> None:
  """Overrides Tilt telemetry.

  By default, Tilt does not send telemetry. After you successfully run a Tiltfile,
  the Tilt web UI will nudge you to opt in or opt out of telemetry.

  The Tiltfile can override these telemetry settings, for teams
  that always want telemetry enabled or disabled.

  Args:
    enable: if true, telemetry will be turned on. If false, telemetry will be turned off.
  """
  pass

def version_settings(check_updates: bool = True, constraint: str = "") -> None:
  """Controls Tilt's behavior with regard to its own version.

  Args:
    check_updates: If true, Tilt will check GitHub for new versions of itself
                   and display a notification in the web UI when an upgrade is
                   available.
    constraint: If non-empty, Tilt will check its currently running version against
                this constraint and generate an error if it doesn't match.
                Examples:

                - `<0.17.0` - less than 0.17.0
                - `>=0.13.2` - at least 0.13.2

                See more at the `constraint syntax documentation <https://github.com/blang/semver#ranges>`_.
  """

def struct(**kwargs) -> Any:
  """Creates an object with arbitrary fields.

  Examples:

  .. code-block:: python

    x = struct(a="foo", b=6)
    print("%s %d" % (x.a, x.b)) # prints "foo 6"
  """


def secret_settings(disable_scrub: bool = False) -> None:
  """Configures Tilt's handling of Kubernetes Secrets. By default, Tilt scrubs
  the text of any Secrets from the logs; e.g. if Tilt applies a Secret with contents
  'mysecurepassword', Tilt redacts this string if ever it appears in the logs,
  to prevent users from accidentally sharing sensitive information in snapshots etc.

  Args:
    disable_scrub: if True, Tilt will *not* scrub secrets from logs.
"""


def update_settings(
    max_parallel_updates: int=3,
    k8s_upsert_timeout_secs: int=30,
    suppress_unused_image_warnings: Union[str, List[str]]=None) -> None:
  """Configures Tilt's updates to your resources. (An update is any execution of or
  change to a resource. Examples of updates include: doing a docker build + deploy to
  Kubernetes; running a live update on an existing container; and executing
  a local resource command).

  Args:
    max_parallel_updates: maximum number of updates Tilt will execute in parallel. Default is 3. Must be a positive integer.
    k8s_upsert_timeout_secs: timeout (in seconds) for Kubernetes upserts (i.e. ``create``/``apply`` calls). Minimum value is 1.
    suppress_unused_image_warnings: suppresses warnings about images that aren't deployed.
      Accepts a list of image names, or '*' to suppress warnings for all images.
"""

def watch_settings(ignore: Union[str, List[str]]) -> None:
  """Configures global watches.

  May be called multiple times to add more ignore patterns.

  Args:
    ignore: A string or list of strings that should not trigger updates. Equivalent to adding
      patterns to .tiltignore. Relative patterns are evaluated relative to the current working dir.
      See `Debugging File Changes <file_changes.html>`_ for more details.
  """


def warn(msg: str) -> None:
  """Emits a warning.

  Warnings are both displayed in the logs and aggregated as alerts.

  Args:
    msg: The message.
  """


def fail(msg: str) -> None:
  """Stops Tiltfile execution and raises an error.

  Can be used anywhere in a Tiltfile.
  If used in a loaded Tiltfile or extension, execution will be stopped up to and including the root Tiltfile.

  See :meth:`exit` to stop execution immediately without triggering an error.

  Args:
    msg: Error message.
  """
  pass


def exit(code: Any) -> None:
  """Stops Tiltfile execution without an error.

  Can be used anywhere in a Tiltfile.
  If used in a loaded Tiltfile or extension, execution will be stopped up to and including the root Tiltfile.

  Requires Tilt v0.22.3+.

  See :meth:`fail` to stop execution immediately and propagate an error.

  Args:
    code: Message or object (will be stringified) to log before halting execution.
  """


def probe(initial_delay_secs: int=0,
          timeout_secs: int=1,
          period_secs: int=10,
          success_threshold: int=1,
          failure_threshold: int=3,
          exec: Optional[ExecAction]=None,
          http_get: Optional[HTTPGetAction]=None,
          tcp_socket: Optional[TCPSocketAction]=None) -> Probe:
  """Creates a :class:`Probe` for use with local_resource readiness checks.

  Exactly one of exec, http_get, or tcp_socket must be specified.

  Args:
    initial_delay_secs: Number of seconds after the resource has started before the probe is
      first initiated (default is 0).
    timeout_secs: Number of seconds after which probe execution is aborted and it is
      considered to have failed (default is 1, must be greater than 0).
    period_secs: How often in seconds to perform the probe (default is 10, must be greater than 0).
    success_threshold: Minimum number of consecutive successes for the result to be
      considered successful after having failed (default is 1, must be greater than 0).
    failure_threshold: Minimum number of consecutive failures for the result to be
      considered failing after having succeeded (default is 3, must be greater than 0).
    exec: Process execution handler to determine probe success.
    http_get: HTTP GET handler to determine probe success.
    tcp_socket: TCP socket connection handler to determine probe success.
  """

def exec_action(command: List[str]) -> ExecAction:
  """Creates an :class:`ExecAction` for use with a :class:`Probe` that runs a command
  to determine service readiness based on exit code.

  The probe is successful if the process terminates normally with an exit code of 0
  within the timeout.

  Args:
    command: Command with arguments to execute.
  """
  pass


def http_get_action(port: int, host: str='localhost', scheme: str='http', path: str='') -> HTTPGetAction:
  """Creates a :class:`HTTPGetAction` for use with a :class:`Probe` that performs an HTTP GET
  request to determine service readiness based on response status code.

  The probe is successful if a valid HTTP response is received within the timeout and has a
  status code >= 200 and < 400.

  Args:
    host: Hostname to use for HTTP request.
    port: Port to use for HTTP request.
    scheme: URI scheme to use for HTTP request, valid values are `http` and `https`.
    path: URI path for HTTP reqeust.
  """
  pass


def tcp_socket_action(port: int, host: str='localhost') -> TCPSocketAction:
  """Creates a :class:`TCPSocketAction` for use with a :class:`Probe` that establishes a TCP
  socket connection to determine service readiness.

  The probe is successful if a TCP socket can be established within the timeout. No data is
  sent or read from the socket.

  Args:
    host: Hostname to use for TCP socket connection.
    port: Port to use for TCP socket connection.
  """
  pass
