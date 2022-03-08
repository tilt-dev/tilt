package query

import (
	_ "embed"

	sitter "github.com/smacker/go-tree-sitter"
)

// FunctionParameters extracts parameters from a function definition and
// supports a mixture of positional parameters, default value parameters,
// typed parameters*, and typed default value parameters*.
//
// * These are not valid Starlark, but we support them to enable using Python
//   type-stub files for improved editor experience.
//go:embed parameters.scm
var FunctionParameters []byte

// Extract all identifiers from the subtree. Include an extra empty identifier
// "" if there is an error node with a trailing period.
//
//go:embed identifiers.scm
var Identifiers []byte

func LeafNodes(node *sitter.Node) []*sitter.Node {
	nodes := []*sitter.Node{}
	Query(node, []byte(`_ @node`), func(q *sitter.Query, match *sitter.QueryMatch) bool {
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
