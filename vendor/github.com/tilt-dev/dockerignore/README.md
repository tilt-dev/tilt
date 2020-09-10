# dockerignore

[![Build Status](https://circleci.com/gh/tilt-dev/dockerignore/tree/master.svg?style=shield)](https://circleci.com/gh/tilt-dev/dockerignore)
[![PkgGoDev](https://pkg.go.dev/badge/github.com/tilt-dev/dockerignore)](https://pkg.go.dev/github.com/tilt-dev/dockerignore)

A fork of Docker's package for reading and interpreting .dockerignore files

## Why

[Tilt](https://tilt.dev/) watches files and live-updates containers when they change.

To do this, Tilt needs to understand container inputs (Docker contexts, dockerignores, etc.)

In the beginning, we simply used Docker as a library.

Over time, we wanted to be able to:

- Fix bugs (for example, https://github.com/moby/moby/issues/41433)

- Provide better debugging tools over Docker contexts (for example, to be able
  to tell you why a file is included or ignored)

- Allow better optimizations (for example, being able to skip a directory that's ignored)

This library adds features and bug fixes to help.

You're welcome to use it! Ideally, we'd like to see fixes and feature here merged upstream.

## License

Licensed under [the Apache License, Version 2.0](LICENSE)

Originally written by the authors of the Moby Project, https://github.com/moby/moby
