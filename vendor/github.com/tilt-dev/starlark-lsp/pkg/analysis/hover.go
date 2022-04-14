package analysis

import (
	"context"

	sitter "github.com/smacker/go-tree-sitter"

	"go.lsp.dev/protocol"

	"github.com/tilt-dev/starlark-lsp/pkg/document"
	"github.com/tilt-dev/starlark-lsp/pkg/query"
)

func (a *Analyzer) Hover(ctx context.Context, doc document.Document, pos protocol.Position) *protocol.Hover {
	pt := query.PositionToPoint(pos)
	node, ok := query.NodeAtPoint(doc, pt)
	if !ok {
		return nil
	}

	var sig protocol.SignatureInformation
	var hoverNode *sitter.Node
	for cur := node; cur != nil; cur = cur.Parent() {
		if cur.Type() != "call" {
			continue
		}
		// TODO - show hover for vars
		var found bool
		fnName := doc.Content(cur.ChildByFieldName("function"))
		sig, found = a.signatureInformation(doc, node, fnName)
		if found {
			hoverNode = cur
			break
		}
	}

	d := sig.Documentation

	if d == nil {
		return nil
	}

	r := query.NodeRange(hoverNode)
	result := &protocol.Hover{
		Range: &r,
	}

	if mc, ok := d.(protocol.MarkupContent); ok {
		result.Contents = mc
	} else {
		result.Contents = protocol.MarkupContent{
			Kind:  protocol.PlainText,
			Value: d.(string),
		}
	}

	return result
}
