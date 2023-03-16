package analysis

import (
	"context"

	"go.lsp.dev/protocol"

	"github.com/tilt-dev/starlark-lsp/pkg/document"
	"github.com/tilt-dev/starlark-lsp/pkg/query"
)

func (a *Analyzer) Hover(ctx context.Context, doc document.Document, pos protocol.Position) *protocol.Hover {
	pt := query.PositionToPoint(pos)
	nodes, ok := a.nodesAtPointForCompletion(doc, pt)
	if !ok {
		return nil
	}

	symbol := a.SymbolAtPosition(doc, pos)

	if symbol.Name == "" {
		return nil
	}

	r := query.NodesRange(nodes)
	result := &protocol.Hover{
		Range: &r,
		Contents: protocol.MarkupContent{
			Kind:  protocol.Markdown,
			Value: symbol.Detail,
		},
	}
	return result
}
