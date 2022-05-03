[![PkgGoDev](https://pkg.go.dev/badge/github.com/looplab/tarjan)](https://pkg.go.dev/github.com/looplab/tarjan)
![Bulid Status](https://github.com/looplab/tarjan/actions/workflows/main.yml/badge.svg)
[![Coverage Status](https://img.shields.io/coveralls/looplab/tarjan.svg)](https://coveralls.io/r/looplab/tarjan)
[![Go Report Card](https://goreportcard.com/badge/looplab/tarjan)](https://goreportcard.com/report/looplab/tarjan)

# Tarjan

Tarjan is a graph loop detection function using Tarjan's algorithm.

The algorithm takes a input graph and produces a slice where each item is a slice of strongly connected vertices. The input graph is in form of a map where the key is a graph vertex and the value is the edges in for of a slice of vertices.

Algorithm description:
http://en.wikipedia.org/wiki/Tarjan’s_strongly_connected_components_algorithm

Based on an implementation by Gustavo Niemeyer (in mgo/txn):
http://bazaar.launchpad.net/+branch/mgo/v2/view/head:/txn/tarjan.go

Gustavo Niemeyer: http://labix.org

For API docs and examples see http://godoc.org/github.com/looplab/tarjan

# Example

```go
graph := make(map[interface{}][]interface{})
graph["1"] = []interface{}{"2"}
graph["2"] = []interface{}{"3"}
graph["3"] = []interface{}{"1"}
graph["4"] = []interface{}{"2", "3", "5"}
graph["5"] = []interface{}{"4", "6"}
graph["6"] = []interface{}{"3", "7"}
graph["7"] = []interface{}{"6"}
graph["8"] = []interface{}{"5", "7", "8"}

output := Connections(graph)
fmt.Println(output)

// Output:
// [[3 2 1] [7 6] [5 4] [8]]
```

# License

Tarjan is licensed under Apache License 2.0

http://www.apache.org/licenses/LICENSE-2.0
