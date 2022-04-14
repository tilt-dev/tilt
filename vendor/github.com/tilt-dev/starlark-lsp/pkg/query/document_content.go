package query

import (
	sitter "github.com/smacker/go-tree-sitter"
)

type DocumentContent interface {
	Input() []byte
	Content(n *sitter.Node) string
	ContentRange(r sitter.Range) string
	Tree() *sitter.Tree
}
