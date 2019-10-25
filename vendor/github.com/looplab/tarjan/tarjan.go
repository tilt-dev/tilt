// Copyright (c) 2013 - Max Persson <max@looplab.se>
// Copyright (c) 2010-2013 - Gustavo Niemeyer <gustavo@niemeyer.net>
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Package tarjan implements a graph loop detection algorithm called Tarjan's algorithm.
//
// The algorithm takes a input graph and produces a slice where each item is a
// slice of strongly connected vertices. The input graph is in form of a map
// where the key is a graph vertex and the value is the edges in for of a slice
// of vertices.
//
// Algorithm description:
// http://en.wikipedia.org/wiki/Tarjan’s_strongly_connected_components_algorithm
//
// Based on an implementation by Gustavo Niemeyer (in mgo/txn):
// http://bazaar.launchpad.net/+branch/mgo/v2/view/head:/txn/tarjan.go
//
package tarjan

// Connections creates a slice where each item is a slice of strongly connected vertices.
//
// If a slice item contains only one vertex there are no loops. A loop on the
// vertex itself is also a connected group.
//
// The example shows the same graph as in the Wikipedia article.
func Connections(graph map[interface{}][]interface{}) [][]interface{} {
	g := &data{
		graph: graph,
		nodes: make([]node, 0, len(graph)),
		index: make(map[interface{}]int, len(graph)),
	}
	for v := range g.graph {
		if _, ok := g.index[v]; !ok {
			g.strongConnect(v)
		}
	}
	return g.output
}

// data contains all common data for a single operation.
type data struct {
	graph  map[interface{}][]interface{}
	nodes  []node
	stack  []interface{}
	index  map[interface{}]int
	output [][]interface{}
}

// node stores data for a single vertex in the connection process.
type node struct {
	lowlink int
	stacked bool
}

// strongConnect runs Tarjan's algorithm recursivley and outputs a grouping of
// strongly connected vertices.
func (data *data) strongConnect(v interface{}) *node {
	index := len(data.nodes)
	data.index[v] = index
	data.stack = append(data.stack, v)
	data.nodes = append(data.nodes, node{lowlink: index, stacked: true})
	node := &data.nodes[index]

	for _, w := range data.graph[v] {
		i, seen := data.index[w]
		if !seen {
			n := data.strongConnect(w)
			if n.lowlink < node.lowlink {
				node.lowlink = n.lowlink
			}
		} else if data.nodes[i].stacked {
			if i < node.lowlink {
				node.lowlink = i
			}
		}
	}

	if node.lowlink == index {
		var vertices []interface{}
		i := len(data.stack) - 1
		for {
			w := data.stack[i]
			stackIndex := data.index[w]
			data.nodes[stackIndex].stacked = false
			vertices = append(vertices, w)
			if stackIndex == index {
				break
			}
			i--
		}
		data.stack = data.stack[:i]
		data.output = append(data.output, vertices)
	}

	return node
}
