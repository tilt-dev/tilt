# dockerignore
[![GoDoc](https://godoc.org/github.com/codeskyblue/dockerignore?status.svg)](https://godoc.org/github.com/codeskyblue/dockerignore)

go library parse gitignore file, source code most from [docker](https://github.com/docker/docker)

## Usage
```go
package main

import (
    "bytes"
    "io/ioutil"
    "log"

    ignore "github.com/codeskyblue/dockerignore"
)

func main() {
    // patterns, err := ignore.ReadIgnoreFile(".gitignore")
    rd := ioutil.NopCloser(bytes.NewBufferString("*.exe"))
    patterns, err := ignore.ReadIgnore(rd)
    if err != nil {
        log.Fatal(err)
    }   
    isSkip, err := ignore.Matches("hello.exe", patterns)
    if err != nil {
        log.Fatal(err)
    }   
    log.Printf("Should skipped true, got %v", isSkip)
}
```

## Rules
The Go lib interprets a `.dockerignore` like file as a newline-separated list of patterns similar to the file globs of Unix shells. 
For the purposes of matching, the root of the context is considered to be both the working and the root directory. 
For example, the patterns /foo/bar and foo/bar both exclude a file or directory named bar in the foo subdirectory of PATH or in the root of the git repository located at URL. 
Neither excludes anything else.

Here is an example .dockerignore file:

    */temp*
    */*/temp*
    temp?

This file causes the following build behavior:

Rule        | Behavior
------------|----------
`*/temp*`   | Exclude files and directories whose names start with temp in any immediate subdirectory of the root. For example, the plain file `/somedir/temporary.txt` is excluded, as is the directory `/somedir/temp`.
`*/*/temp*` | Exclude files and directories starting with temp from any subdirectory that is two levels below the root. For example, `/somedir/subdir/temporary.txt` is excluded.
`temp?`     | Exclude files and directories in the root directory whose names are a one-character extension of temp. For example, `/tempa` and `/tempb` are excluded.

Matching is done using Go’s filepath.Match rules. 
A preprocessing step removes leading and trailing whitespace and eliminates `.` and `..` elements using Go’s filepath.Clean. 
Lines that are blank after preprocessing are ignored.

Lines starting with `!` (exclamation mark) can be used to make exceptions to exclusions.
The following is an example `.dockerignore` file that uses this mechanism:

    *.md
    !README.md

All markdown files except `README.md` are excluded from the context.

The placement of `!` exception rules influences the behavior: the last line of the `.dockerignore` that matches a particular file determines whether it is included or excluded. 
Consider the following example:

    *.md
    !README*.md
    README-secret.md

No markdown files are included in the context except README files other than `README-secret.md`

Now consider this example:

    *.md
    README-secret.md
    !README*.md

All of the README files are included.
The middle line has no effect because `!README*.md` matches `README-secret.md` and comes last.

You can even use the `.dockerignore` file to exclude the Dockerfile and `.dockerignore` files.
These files are still sent to the daemon because it needs them to do its job.
But the ADD and COPY commands do not copy them to the the image.

Finally, you may want to specify which files to include in the context, rather than which to exclude.
To achieve this, specify `*` as the first pattern, followed by one or more `!` exception patterns.

Note: For historical reasons, the pattern `.` is ignored.

## LICENCE
Folow the docker license, this lib use [APACHE V2 LICENSE](LICENSE)
