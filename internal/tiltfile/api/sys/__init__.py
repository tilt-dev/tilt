from typing import List

argv: List[str] = []
"""The list of command line arguments passed to Tilt on start.

`argv[0]` is the Tilt binary name.
"""

executable: str = ""
"""A string giving the absolute path of the Tilt binary.

Based on how Tilt was originally invoked. There is no guarantee that
the path is still pointing to a valid Tilt binary. If the path has
a symlink, the behavior is operating system depdendent.
"""
