# go-jsonpointer

[![Build Status](https://travis-ci.org/mattn/go-jsonpointer.png?branch=master)](https://travis-ci.org/mattn/go-jsonpointer)
[![Codecov](https://codecov.io/gh/mattn/go-jsonpointer/branch/master/graph/badge.svg)](https://codecov.io/gh/mattn/go-jsonpointer)
[![GoDoc](https://godoc.org/github.com/mattn/go-jsonpointer?status.svg)](http://godoc.org/github.com/mattn/go-jsonpointer)
[![Go Report Card](https://goreportcard.com/badge/github.com/mattn/go-jsonpointer)](https://goreportcard.com/report/github.com/mattn/go-jsonpointer)

Go implementation of JSON Pointer (RFC6901)

## Usage

`jsonpointer.Get(obj, pointer)`
```go
json := `
{
	"foo": [1,true,2]
}
`
var obj interface{}
json.Unmarshal([]byte(json), &obj)
jsonpointer.Get(obj, "/foo/1") // Should be true
```

`jsonpointer.Set(obj, pointer, newvalue)`
```go
json := `
{
	"foo": [1,true,2]
}
`
var obj interface{}
json.Unmarshal([]byte(json), &obj)
jsonpointer.Set(obj, "/foo/1", false)
// obj should be {"foo":[1,false,2]}
```

`jsonpointer.Remove(obj, pointer)`
```go
json := `
{
	"foo": [1,true,2]
}
`
var obj interface{}
json.Unmarshal([]byte(json), &obj)
jsonpointer.Remove(obj, "/foo/1")
// obj should be {"foo":[1,2]}
```

## Installation

```
$ go get github.com/mattn/go-jsonpointer
```

## License

MIT

## Author

Yasuhiro Matsumoto (a.k.a mattn)
