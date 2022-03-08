from typing import Dict

name: str = ""
"""The name of the operating system. 'posix' (for Linux and MacOS) or 'nt' (for Windows).

Designed for consistency with
`os.name in Python <https://docs.python.org/3/library/os.html#os.name>`_.
"""

environ = Dict[str, str]
"""A dictionary of your environment variables.

For example, ``os.environ['HOME']`` is usually your home directory.

Captured each time the Tiltfile begins execution.

Tiltfile dictionaries support many of the same methods
as Python dictionaries, including:

- dict.get(key, default)
- dict.items()

See the `Starlark spec <https://github.com/bazelbuild/starlark/blob/master/spec.md#built-in-methods>`_ for more.
"""

def getcwd() -> str:
  """Returns a string representation of the current working directory.

  The current working directory is the directory containing the currently executing Tiltfile.
  If your Tiltfile runs any commands, they run from this directory.

  While calling :meth:load or :meth:include to execute another Tiltfile,
  returns the directory of the loaded/included Tiltfile.
  """
  pass

def getenv(key: str, default=None) -> str:
  """Return the value of the environment variable key if it exists, or default if it doesn’t.

  Args:
    key: An environment variable name.
    default: The value to return if the variable doesn't exist.
  """
  pass

def putenv(key: str, value: str):
  """Set the environment variable named key to the string value. Takes effect
  immediately in the Tilt process. Any new subprocesses will have this
  environment value.

  Args:
    key: An environment variable name.
    value: The new value.
  """
  pass

def unsetenv(key: str, value: str):
  """Delete the environment variable named key. Takes effect immediately in the
  Tilt process. Any new subprocesses will not have this variable.

  Args:
    key: An environment variable name.
  """
  pass
