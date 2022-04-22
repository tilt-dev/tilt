package analysis

import (
	"go.lsp.dev/protocol"

	"github.com/tilt-dev/starlark-lsp/pkg/document"
	"github.com/tilt-dev/starlark-lsp/pkg/query"
)

func (a Analyzer) SymbolAtPosition(doc document.Document, pos protocol.Position) query.Symbol {
	var result query.Symbol
	pt := query.PositionToPoint(pos)
	nodes, ok := a.nodesAtPointForCompletion(doc, pt)
	if !ok {
		return result
	}

	limit := nodes[len(nodes)-1].EndPoint()
	symbols := a.completeExpression(doc, nodes, limit)
	identifiers := query.ExtractIdentifiers(doc, nodes, &limit)
	if len(identifiers) == 0 {
		return result
	}
	for _, s := range symbols {
		if s.Name == identifiers[len(identifiers)-1] {
			result = s
		}
	}
	return result
}
