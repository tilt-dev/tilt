# go-get

A repository fetcher, forked from golang/go

[![Build Status](https://circleci.com/gh/tilt-dev/go-get/tree/master.svg?style=shield)](https://circleci.com/gh/tilt-dev/go-get)
[![PkgGoDev](https://pkg.go.dev/badge/github.com/tilt-dev/go-get)](https://pkg.go.dev/github.com/tilt-dev/go-get)

## Why?

### When in doubt, simply port Go's source code, documentation, and tests.

- from *Deno Standard Modules*, https://deno.land/std@0.68.0

[Tilt](https://tilt.dev/) needs a system for importing extensions.

We love the Go package-import system.

We decided to copy it!

But when we looked at how Go's `go get` was implemented, 
we saw that it supports a lot of different repositories.

This package contains a fork of that package, to make it easier to re-use.

## How?

```
package get_test

import (
	"fmt"
	"os/user"
	"path/filepath"
	"testing"

	"github.com/tilt-dev/go-get"
)

func ExampleGet(t *testing.T) {
	usr, _ := user.Current()
	downloader := get.NewDownloader(filepath.Join(usr.HomeDir, ".cache", "go-get"))
	path, err := downloader.Download("github.com/tilt-dev/tilt-extensions/hello_world")
	if err != nil {
		t.Fatal(err)
	}

	fmt.Printf("Downloaded to %s\n", path)
}
```

## License

Licensed under [3-clause BSD](LICENSE)

Originally Copyright (c) 2009 The Go Authors. All rights reserved.

Modified by Windmill Engineering, Inc.
