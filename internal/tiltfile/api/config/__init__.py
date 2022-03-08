from typing import Dict, Union, List, Callable, Any

tilt_subcommand: str = ''
"""The sub-command with which `tilt` was invoked. Does not include extra args or options.

Examples:

- run `tilt down` -> `config.tilt_subcommand == "down"`
- run `tilt up frontend backend` -> `config.tilt_subcommand == "up"`
- run `tilt alpha tiltfile-result` -> `config.tilt_subcommand == "alpha tiltfile-result"`
"""

main_path: str = ''
"""The absolute path of the main Tiltfile."""

main_dir: str = ''
"""The absolute directory of the main Tiltfile.

Often used to determine the location of vendored code and caches."""

def define_string_list(name: str, args: bool=False, usage: str="") -> None:
    """
    Defines a config setting of type `List[str]`.

    Allows the user invoking Tilt to configure a key named ``name`` to be in the
    dict returned by :meth:`parse`.

    See the `Tiltfile config documentation <tiltfile_config.html>`_ for examples
    and more information.

    Args:
      name: The name of the config setting
      args: If False, the config setting is specified by its name. (e.g., if it's named "foo",
            ``tilt up -- --foo bar`` this setting would be ``["bar"]``.)

            If True, the config setting is specified by unnamed positional args. (e.g.,
            in ``tilt up -- 1 2 3``, this setting would be ``["1" "2" "3"]``.)
      usage: When arg parsing fails, what to print for this setting's description.
    """

def define_string(name: str, args: bool=False, usage: str="") -> None:
    """
    Defines a config setting of type `str`.

    Allows the user invoking Tilt to configure a key named ``name`` to be in the
    dict returned by :meth:`parse`.

    For instance, at runtime, to set a flag of this type named `foo` to value "bar", run ``tilt up -- --foo bar``.

    See the `Tiltfile config documentation <tiltfile_config.html>`_ for examples
    and more information.

    Args:
      name: The name of the config setting
      args: If False, the config setting is specified by its name. (e.g., if it's named "foo",
            ``tilt up -- --foo bar`` this setting would be ``"bar"``.)

            If True, the config setting is specified by unnamed positional args. (e.g.,
            in ``tilt up -- 1``, this setting would be ``"1"``.)
      usage: When arg parsing fails, what to print for this setting's description.
    """

def define_bool(name: str, args: bool=False, usage: str="") -> None:
    """
    Defines a config setting of type `bool`.

    Allows the user invoking Tilt to configure a key named ``name`` to be in the
    dict returned by :meth:`parse`.

    For instance, at runtime, to set a flag of this type named `foo` to value `True`, run ``tilt up -- --foo``.
    To set a value to ``False``, you can run ``tilt up -- --foo=False``, or use a default value, e.g.:
    ```python
    config.define_bool('foo')
    cfg = config.parse()
    do_stuff = cfg.get('foo', False)
    ```

    See the `Tiltfile config documentation <tiltfile_config.html>`_ for examples
    and more information.

    Args:
      name: The name of the config setting
      args: If False, the config setting is specified by its name. (e.g., if it's named "foo",
            ``tilt up -- --foo`` this setting would be ``True``.)

            If True, the config setting is specified by unnamed positional args. (e.g.,
            in ``tilt up -- True``, this setting would be ``True``.) (This usage
            isn't likely to be what you want)
      usage: When arg parsing fails, what to print for this setting's description.
    """

def parse() -> Dict[str, Any]:
    """
    Loads config settings from tilt_config.json, overlays config settings from
    Tiltfile command-line args, validates them using the setting definitions
    specified in the Tiltfile, and returns a Dict of the resulting settings.

    Settings that are defined in the Tiltfile but not specified in the config
    file or command-line args will be absent from the dict. Access values via,
    e.g., `cfg.get('foo', ["hello"])` to have a default value.

    Note: by default, Tilt interprets the Tilt command-line args as the names of
    Tilt resources to run. When a Tiltfile calls :meth:`parse`, that behavior is
    suppressed, since those args are now managed by :meth:parse. If a
    Tiltfile uses :meth:`parse` and also needs to allow specifying a set
    of resources to run, it needs to call :meth:`set_enabled_resources`.

    See the `Tiltfile config documentation <tiltfile_config.html>`_ for examples
    and more information.

    Returns:
      A Dict where the keys are settings names and the values are their values.
    """

def set_enabled_resources(resources: List[str]) -> None:
    """
    Tells Tilt to only run the specified resources.
    (takes precedence over the default behavior of "run the resources specified
    on the command line")

    Calling this with an empty list results in all resources being run.

    See the `Tiltfile config documentation <tiltfile_config.html>`_ for examples
    and more information.

    Args:
      resources: The names of the resources to run, or an empty list to run them
                 all.
    """

def clear_enabled_resources() -> None:
    """
    Tells Tilt that all resources should be disabled. This allows the user to manually enable only the resources they want once Tilt is running.
    """
