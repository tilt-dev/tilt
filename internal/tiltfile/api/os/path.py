def abspath(path: str) -> str:
  """Return a normalized, absolute version of the path, relative to the current working directory.

  Args:
    path: A filesystem path
"""
  pass

def relpath(targpath: str, basepath: str='') -> str:
  """Return the path of `targpath` relative to `basepath` (by default, relative to CWD).
  On success, the returned path will always be relative to `basepath`, even if `basepath`
  and `targpath` share no elements. An error is raised if `targpath` can't be made relative to `basepath`.

  e.g. for the Tiltfile at `/code/Tiltfile`:

  * `relpath('/code/foo/bar')` --> `foo/bar`
  * `relpath('/code/foo/bar', '/code/foo')` --> `bar`
  * `relpath('/code/foo', '/code/baz')` --> `../foo`
  * `relpath('/code/foo', 'other/path')` --> error

  Args:
    targpath: Filesystem path to be made relative
    basepath: Filesystem path (defaults to the current working directory)
"""
  pass

def basename(path: str) -> str:
  """Return the basename of the path.

  Args:
    path: A filesystem path
"""
  pass

def dirname(path: str) -> str:
  """Return the directory name of the path.

  Args:
    path: A filesystem path
"""
  pass

def exists(path: str) -> bool:
  """Checks if a file or directory exists at the specified path.

  Returns false if this is a broken symlink, or if the user doesn't have permission
  to stat the file at this path.

  On Tilt v0.18.3 and below, watches the path, and reloads the Tiltfile if the contents change.

  On Tilt v0.18.4 and up, does no watching.

  Args:
    path: A filesystem path
"""
  pass

def join(path, *paths: str) -> str:
  """Join one or more path components with the OS-specific file separator.

  Args:
    path: A filesystem path component
    paths: A variable list of components to join
"""
  pass

def realpath(path: str) -> str:
  """Return the canonical path of the specified filename, eliminating any symbolic links encountered in the path (if they are supported by the operating system).

  Args:
    path: A filesystem path
"""
  pass
