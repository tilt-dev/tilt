realpath
========

[![Travis last test result on master][travis-shield]][travis-link]

This is a implementation of realpath() function in Go (golang).

If you provide it with a valid relative path / alias path, it will return you
with a string of its real absolute path in the system.

The original version is created by Taru Karttunen in golang-nuts group. You
may read [the original post about it](https://groups.google.com/forum/?fromgroups#!topic/golang-nuts/htns6YWMp7s).

[travis-shield]: https://api.travis-ci.org/yookoala/realpath.svg?branch=master
[travis-link]: https://travis-ci.org/yookoala/realpath?branch=master

Installation
------------

```
go get github.com/yookoala/realpath
```


Usage
-----

```go
import "github.com/yookoala/realpath"

func main() {
	myRealpath, err := realpath.Realpath("/some/path")
}
```


License
-------
The MIT License

Copyright (c) 2012-2017 [Taru Karttunen](https://github.com/taruti/) and
[Koala Yeung](https://github.com/yookoala/)

Permission is hereby granted, free of charge, to any person obtaining a copy
of this software and associated documentation files (the "Software"), to deal
in the Software without restriction, including without limitation the rights
to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
copies of the Software, and to permit persons to whom the Software is
furnished to do so, subject to the following conditions:

The above copyright notice and this permission notice shall be included in
all copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN
THE SOFTWARE.
