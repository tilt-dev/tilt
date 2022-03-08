package document

import sitter "github.com/smacker/go-tree-sitter"

type Document interface {
	Content(n *sitter.Node) string
	ContentRange(r sitter.Range) string

	Tree() *sitter.Tree

	Copy() Document

	Close()
}

type NewDocumentFunc func(input []byte, tree *sitter.Tree) Document

func NewDocument(input []byte, tree *sitter.Tree) Document {
	return document{
		input: input,
		tree:  tree,
	}
}

type document struct {
	// input is the file as it exists in the editor buffer.
	input []byte

	// tree represents the parsed version of the document.
	tree *sitter.Tree
}

var _ Document = document{}

func (d document) Content(n *sitter.Node) string {
	return n.Content(d.input)
}

func (d document) ContentRange(r sitter.Range) string {
	return string(d.input[r.StartByte:r.EndByte])
}

func (d document) Tree() *sitter.Tree {
	return d.tree
}

func (d document) Close() {
	d.tree.Close()
}

// Copy creates a shallow copy of the Document.
//
// The Contents byte slice is returned as-is.
// A shallow copy of the Tree is made, as Tree-sitter trees are not thread-safe.
func (d document) Copy() Document {
	return document{
		input: d.input,
		tree:  d.tree.Copy(),
	}
}
