from typing import Dict, Union, List, Callable

class LocalPath:
  """A path on disk"""

class Blob:
  """The result of executing a command on your local system"""

class Yaml:
  """A string that represents YAML that can be parsed"""

class Repo:
  """Represents a version control repository"""
  def path(self, path: str) -> LocalPath:
    """Returns the absolute path to the file specified at ``path`` in the repo.
    path must be a relative path.

    Respects ``.gitignore``.

    Args:
      path: relative path in repository
    Returns:
      A LocalPath resource, representing a local path on disk.
    """
    pass

def local_git_repo(path: str) -> Repo:
  """Creates a ``repo`` from the git repo at ``path``."""
  pass

def docker_build(ref: str, context: str, build_args: Dict[str, str] = {}, dockerfile: Union[str, LocalPath] = "Dockerfile", dockerfile_contents: Union[str, Blob] = "") -> None:
  """Builds a docker image.

  Note that you can't set both the `dockerfile` and `dockerfile_contents` arguments (will throw an error).

  Example: ``docker_build('myregistry/myproj/backend', '/path/to/code')`` is roughly equivalent to the call ``docker build /path/to/code -t myregistry/myproj/backend``

  Args:
    ref: name for this image (e.g. 'myproj/backend' or 'myregistry/myproj/backend'). If this image will be used in a k8s resource(s), this ref must match the ``spec.container.image`` param for that resource(s).
    context: path to use as the Docker build context.
    build_args: build-time variables that are accessed like regular environment variables in the ``RUN`` instruction of the Dockerfile. See `the Docker Build Arg documentation <https://docs.docker.com/engine/reference/commandline/build/#set-build-time-variables---build-arg>`_
    dockerfile: path to the Dockerfile to build (may be absolute, or relative to cwd)
    dockerfile_contents: raw contents of the Dockerfile to use for this build
  """
  pass

class FastBuild:
  """An image that was created with ```fast_build```"""
  def add(src: Union[LocalPath, Repo], dest: str) -> 'FastBuild':
    """Adds the content from ``src`` into the image at path ``dest``."""
    pass

  def run(cmd: str, trigger: Union[List[str], str] = []) -> None:
    """Runs ``cmd`` as a build step in the image.

    Args:
      cmd: A shell command.
      trigger: If the ``trigger`` argument is specified, the build step is only run on changes to the given file(s).
    """
    pass

  def hot_reload() -> None:
    """Setting this on a ``FastBuild`` image tells Tilt that this container knows how to automatically reload any changes in the container. As a result there is no need to restart it.

    This is useful for containers that run something like nodemon or webpack Hot Module Replacement to update running processes quickly."""
    pass


def fast_build(img_name: str, dockerfile_path: str, entrypoint: str = "") -> FastBuild:
  """Initiates a docker image build that supports ``add`` s and ``run`` s, and that uses a cache for subsequent builds.

    See the `fast build documentation <https://docs.tilt.build/fast_build.html>`_.
  """
  pass

def k8s_yaml(yaml: Union[str, List[str], LocalPath, Blob]) -> None:
  """Call this with a path to a file that contains YAML, or with a ``Blob`` of YAML.

  We will infer what (if any) of the k8s resources defined in your YAML
  correspond to Images defined elsewhere in your ``Tiltfile`` (matching based on
  the DockerImage ref and on pod selectors). Any remaining YAML is YAML that Tilt
  applies to your k8s cluster independently.

  Args:
    yaml: Path(s) to YAML or YAML as a ``Blob``.
  """
  pass

def k8s_resource(name: str, yaml: Union[str, Blob] = "", image: str = "",
    port_forwards: Union[str, int, List[int]] = [], extra_pod_selectors: Union[Dict[str, str], List[Dict[str, str]]] = []) -> None:
  """Creates a kubernetes resource that tilt can deploy using the specified image.

  Args:
    name: What call this resource in the UI
    yaml: Optional YAML. If YAML, as a string or Blob is
      not passed we expect to be able to extract it from an
      existing resource.
    image: An optional Image. If the image is not passed,
      we expect to be able to extract it from an existing resource.
    port_forwards: Local ports to connect to the pod. If no
      target port is specified, will use the first container port.
      Example values: 9000 (connect localhost:9000 to the default container port),
      '9000:8000' (connect localhost:9000 to the container port 8000),
      ['9000:8000', '9001:8001'] (connect localhost:9000 and :9001 to the
      container ports 8000 and 8001, respectively).
    extra_pod_selectors: In addition to relying on Tilt's heuristics to automatically
      find K8S resources associated with this resource, a user may specify extra
      labelsets to force entities to be associated with this resource. An entity
      will be associated with this resource if it has all of the labels in at
      least one of the entries specified (but still also if it meets any of
      Tilt's usual mechanisms).
  """
  pass

def local(cmd: str) -> Blob:
  """Runs cmd, waits for it to finish, and returns its stdout as a Blob"""
  pass

def read_file(file_path: Union[str, LocalPath]) -> Blob:
  """Reads file and returns its contents.

  Args:
    file_path: Path to the file locally"""
  pass


def kustomize(pathToDir: str) -> Blob:
  """Run `kustomize <https://github.com/kubernetes-sigs/kustomize>`_ on a given directory and return the resulting YAML as a Blob"""
  pass

def helm(pathToChartDir: Union[str, LocalPath]) -> Blob:
  """Run `helm template <https://docs.helm.sh/helm/#helm-template>`_ on a given directory that contains a chart and return the fully rendered YAML as a Blob"""

def fail(msg: str) -> None:
  """Raises an error that cannot be intercepted. Can be used anywhere in a Tiltfile."""

def yaml(contents: str) -> Yaml:
  """Creates a Yaml object that wraps the provided string. Useful for passing strings in to functions that expect YAML like ``k8s_yaml``"""
