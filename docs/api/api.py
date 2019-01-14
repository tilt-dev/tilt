from typing import Dict, Union, List, Callable

class LocalPath:
  """A path on disk"""

class Blob:
  """The result of executing a command on your local system"""

class Repo:
  def path(self, path: str) -> LocalPath:
    """Returns the absolute path to the file specified at ``path`` in the repo.
    path must be a relative path.

    Args:
      path: relative path in repository
    Returns:
      A LocalPath resource, representing a local path on disk.
    """
    pass

def local_git_repo(path: str) -> Repo:
  """Creates a ``repo`` from the git repo at ``path``."""
  pass

def docker_build(ref: str, context: str, build_args: Dict[str, str] = {}, dockerfile: str = "Dockerfile") -> None:
  """Builds a docker image.

  Args:
    ref: e.g. a blorgdev/backend or gcr.io/project-name/bucket-name
    context: The path to use as the Docker build context.
    build_args: the build-time variables that are accessed like regular environment variables in the ``RUN`` instruction of the Dockerfile. See `the Docker Build Arg documentation <https://docs.docker.com/engine/reference/commandline/build/#set-build-time-variables---build-arg>`_
    dockerfile: The path to a Dockerfile
  """
pass

class FastBuild:
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

def k8s_resource(name: str, yaml: Union[str, Blob] = "", image: str = "", port_forwards: Union[str, int, List[int]] = []) -> None:
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
