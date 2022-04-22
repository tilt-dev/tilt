package query

import (
	sitter "github.com/smacker/go-tree-sitter"
	"go.lsp.dev/uri"
)

type DocumentContent interface {
	Input() []byte
	Content(n *sitter.Node) string
	ContentRange(r sitter.Range) string
	Tree() *sitter.Tree
	URI() uri.URI
}
