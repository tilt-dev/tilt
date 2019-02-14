package dockerfile

import (
	"bytes"

	"github.com/moby/buildkit/frontend/dockerfile/parser"
	"github.com/pkg/errors"
)

type AST struct {
	result *parser.Result
}

func ParseAST(df Dockerfile) (AST, error) {
	result, err := parser.Parse(bytes.NewBufferString(string(df)))
	if err != nil {
		return AST{}, errors.Wrap(err, "dockerfile.ParseAST")
	}

	return AST{
		result: result,
	}, nil
}

// Post-order traversal of the Dockerfile AST.
// Halts immediately on error.
func (a AST) Traverse(visit func(*parser.Node) error) error {
	return a.traverseNode(a.result.AST, visit)
}

func (a AST) traverseNode(node *parser.Node, visit func(*parser.Node) error) error {
	for _, c := range node.Children {
		err := a.traverseNode(c, visit)
		if err != nil {
			return err
		}
	}
	return visit(node)
}
