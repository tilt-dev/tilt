package query

import (
	_ "embed"

	sitter "github.com/smacker/go-tree-sitter"
)

func LeafNodes(node *sitter.Node) []*sitter.Node {
	nodes := []*sitter.Node{}
	Query(node, `_ @node`, func(q *sitter.Query, match *sitter.QueryMatch) bool {
		for _, c := range match.Captures {
			if c.Node.Type() == NodeTypeIdentifier ||
				(c.Node.ChildCount() == 0 &&
					(c.Node.Parent() == nil || c.Node.Parent().Type() != NodeTypeIdentifier)) {
				nodes = append(nodes, c.Node)
			}
		}
		return true
	})
	return nodes
}

func LoadStatements(input []byte, tree *sitter.Tree) []*sitter.Node {
	nodes := []*sitter.Node{}
	Query(tree.RootNode(), `(call) @call`, func(q *sitter.Query, match *sitter.QueryMatch) bool {
		for _, c := range match.Captures {
			id := c.Node.ChildByFieldName("function")
			name := id.Content(input)
			if name == "load" {
				nodes = append(nodes, c.Node)
			}
		}
		return true
	})
	return nodes
}
