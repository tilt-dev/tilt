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

	symbols := a.completeExpression(doc, nodes, pt)
	var symbol protocol.DocumentSymbol
	limit := nodes[len(nodes)-1].EndPoint()
	identifiers := query.ExtractIdentifiers(doc, nodes, &limit)
	if len(identifiers) == 0 {
		return nil
	}
	for _, s := range symbols {
		if s.Name == identifiers[len(identifiers)-1] {
			symbol = s
		}
	}

	if symbol.Name == "" {
		return nil
	}

	r := query.NodesRange(nodes)
	result := &protocol.Hover{
		Range: &r,
		Contents: protocol.MarkupContent{
			Kind:  protocol.PlainText,
			Value: symbol.Detail,
		},
	}
	return result
}
