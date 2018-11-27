from typing import Dict, Union, List, Callable

class Image:
  """A built Docker image"""

  def cache(path: str) -> None:
    """Caches the given path between image builds.

    Popular directories to cache include:

    * Go projects: ``/root/.cache/go-build``
    * NodeJS projects: ``/src/node_modules``, ``/src/yarn.lock``

    Args:
      path: The path to cache. May be a file or a directory.
    """
    pass

def static_build(dockerfile: str, ref: str, build_args: Dict[str, str] = {}, context: str = "") -> Image:
  """Builds a docker image.

  Args:
    dockerfile: The path to a Dockerfile
    ref: e.g. a blorgdev/backend or gcr.io/project-name/bucket-name
    build_args: the build-time variables that are accessed like regular environment variables in the ``RUN`` instruction of the Dockerfile. See `the Docker Build Arg documentation <https://docs.docker.com/engine/reference/commandline/build/#set-build-time-variables---build-arg>`_
    context: The path to use as the Docker build context. Defaults to the Dockerfile directory.
  """
  pass

class LocalPath:
  """A path on disk"""

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

def start_fast_build(dockerfile_path: str, img_name: str, entrypoint: str = "") -> None:
  """Initiates a docker image build that supports ``add`` s and ``run`` s, and that uses a cache for subsequent builds.

    See https://docs.tilt.build/fast_build.html
  """
  pass

def add(src: Union[LocalPath, Repo], dest: str) -> None:
  """Adds the content from ``src`` into the image at path ``dest``."""
  pass

def run(cmd: str, trigger: Union[List[str], str] = []) -> None:
  """Runs ``cmd`` as a build step in the image.

  Args:
    cmd: A shell command.
    trigger: If the ``trigger`` argument is specified, the build step is only run on changes to the given file(s).
  """
  pass

class Service:
  """Represents a Kubernetes service that Tilt can deploy and monitor."""

  def port_forward(self, local: int, remote: int = 0):
    """Sets up port-forwarding for the deployed container when it's ready.

    Args:
      local: The local port
      remote: The container port. If not specified, we will forward to the first port in the container
  """
  pass

def global_yaml(yaml: str) -> None:
  """Call this *on the top level of your Tiltfile* with a string of YAML.

  We will infer what (if any) of the k8s resources defined in your YAML
  correspond to ``Services`` defined elsewhere in your ``Tiltfile`` (matching based on
  the DockerImage ref and on pod selectors). Any remaining YAML is *global YAML*,
  i.e. YAML that Tilt applies to your k8s cluster independently of any
  ``Service`` you define.

  Args:
    yaml: YAML as a string
  """
  pass

def k8s_service(img: Image, yaml: str="") -> Service:
  """Creates a kubernetes service that tilt can deploy using the the image passed in.



  Args:
    img: A Docker image.
    yaml: An optional Kubernetes resource YAML. If the YAML is not passed,
      we expect to be able to extract it from :func:`global_yaml`.
  """
  pass

def composite_service(services: List[Callable[[], Service]]) -> Service:
  """Creates a composite service; tilt will deploy (and watch) all services returned by the functions in ``service_fns``."""
  pass

def local(cmd: str) -> str:
  """Runs cmd, waits for it to finish, and returns its stdout."""
  pass

def read_file(file_path: Union[str, LocalPath]) -> str:
  """Reads file and returns its contents.

  Args:
    file_path: Path to the file locally"""
  pass

def stop_build() -> Image:
  """Closes the currently active build (started by :func:`start_fast_build`).

  Returns a container Image that has all of the adds and runs applied."""
  pass

def kustomize(pathToDir: str) -> str:
  """Run `kustomize <https://github.com/kubernetes-sigs/kustomize>`_ on a given directory and return the resulting YAML."""
  pass
