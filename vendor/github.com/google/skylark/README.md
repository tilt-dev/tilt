
<!-- This file is the project homepage at github.com/google/skylark -->

# Skylark in Go

This is the home of the _Skylark in Go_ project.
Skylark in Go is an interpreter for Skylark, implemented in Go.

Skylark is a dialect of Python intended for use as a configuration language.
Like Python, it is an untyped dynamic language with high-level data
types, first-class functions with lexical scope, and garbage collection.
Unlike CPython, independent Skylark threads execute in parallel, so
Skylark workloads scale well on parallel machines.
Skylark is a small and simple language with a familiar and highly
readable syntax. You can use it as an expressive notation for
structured data, defining functions to eliminate repetition, or you
can use it to add scripting capabilities to an existing application.

A Skylark interpreter is typically embedded within a larger
application, and the application may define additional domain-specific
functions and data types beyond those provided by the core language.
For example, Skylark was originally developed for the
[Bazel build tool](https://bazel.build).
Bazel uses Skylark as the notation both for its BUILD files (like
Makefiles, these declare the executables, libraries, and tests in a
directory) and for [its macro
language](https://docs.bazel.build/versions/master/skylark/language.html),
through which Bazel is extended with custom logic to support new
languages and compilers.


## Documentation

* Language definition: [doc/spec.md](doc/spec.md)

* About the Go implementation: [doc/impl.md](doc/impl.md)

* API documentation: [godoc.org/github.com/google/skylark](https://godoc.org/github.com/google/skylark)

* Mailing list: [skylark-go](https://groups.google.com/forum/#!forum/skylark-go)

* Issue tracker: [https://github.com/google/skylark/issues](https://github.com/google/skylark/issues)

* Travis CI: ![Travis CI](https://travis-ci.org/google/skylark.svg) [https://travis-ci.org/google/skylark](https://travis-ci.org/google/skylark)

### Getting started

Build the code:

```shell
$ go get github.com/google/skylark/...
$ go build github.com/google/skylark/cmd/skylark
```

Run the interpreter:

```
$ cat coins.sky
coins = {
  'dime': 10,
  'nickel': 5,
  'penny': 1,
  'quarter': 25,
}
print('By name:\t' + ', '.join(sorted(coins.keys())))
print('By value:\t' + ', '.join(sorted(coins.keys(), key=coins.get)))

$ ./skylark coins.sky
By name:	dime, nickel, penny, quarter
By value:	penny, nickel, dime, quarter
```

Interact with the read-eval-print loop (REPL):

```
$ ./skylark
>>> def fibonacci(n):
...    res = list(range(n))
...    for i in res[2:]:
...        res[i] = res[i-2] + res[i-1]
...    return res
...
>>> fibonacci(10)
[0, 1, 1, 2, 3, 5, 8, 13, 21, 34]
>>>
```

When you have finished, type `Ctrl-D` to close the REPL's input stream. 

### Contributing

We welcome submissions but please let us know what you're working on
if you want to change or add to the Skylark repository.

Before undertaking to write something new for the Skylark project,
please file an issue or claim an existing issue.
All significant changes to the language or to the interpreter's Go
API must be discussed before they can be accepted.
This gives all participants a chance to validate the design and to
avoid duplication of effort.

Despite some differences, the Go implementation of Skylark strives to
match the behavior of the Java implementation used by Bazel.
For that reason, proposals to change the language itself should
generally be directed to the Bazel team, not to the maintainers of
this project.
Only once there is consensus that a language change is desirable may
its Go implementation proceed.

We use GitHub pull requests for contributions.

Please complete Google's contributor license agreement (CLA) before
sending your first change to the project.  If you are the copyright
holder, you will need to agree to the
[individual contributor license agreement](https://cla.developers.google.com/about/google-individual),
which can be completed online.
If your organization is the copyright holder, the organization will
need to agree to the [corporate contributor license agreement](https://cla.developers.google.com/about/google-corporate).
If the copyright holder for your contribution has already completed
the agreement in connection with another Google open source project,
it does not need to be completed again.

### Stability

We reserve the right to make breaking language and API changes at this
stage in the project, although we will endeavor to keep them to a minimum.
Once the project has its long-term name (see below) and location, and
the Bazel team has finalized the version 1 language specification,
we will be more rigorous with interface stability.

### Credits

Skylark was designed and implemented in Java by
Ulf Adams,
Lukács Berki,
Jon Brandvein,
John Field,
Laurent Le Brun,
Dmitry Lomov,
Damien Martin-Guillerez,
Vladimir Moskva, and
Florian Weikert,
standing on the shoulders of the Python community.
The Go implementation was written by Alan Donovan and Jay Conrod;
its scanner was derived from one written by Russ Cox.

### Legal

Skylark in Go is Copyright (c) 2017 The Bazel Authors.
All rights reserved.

It is provided under a 3-clause BSD license:
[LICENSE](https://github.com/google/skylark/blob/master/LICENSE).

The name "Skylark" is a code name of the Bazel project.
We plan to rename the language before the end of 2017 to reflect its
applicability to projects unrelated to Bazel.

Skylark in Go is not an official Google product.
