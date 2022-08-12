package analysis

import (
	"context"

	"go.lsp.dev/protocol"

	"github.com/tilt-dev/starlark-lsp/pkg/document"
	"github.com/tilt-dev/starlark-lsp/pkg/query"
)

func (a *Analyzer) Definition(ctx context.Context, doc document.Document, pos protocol.Position) []protocol.Location {
	symbol := a.SymbolAtPosition(doc, pos)

	// this might be because no matching symbol was found, or because it's a builtin with no location
	if !symbol.HasLocation() {
		return nil
	}

	pt := query.PositionToPoint(pos)

	// if pos is already on the definition, then don't return a navigation destination
	// (it feels weird to get the navigate cursor and click and have it seemingly do nothing)
	if symbol.Location.URI == doc.URI() && query.RangeContainsPoint(query.SitterRange(symbol.Location.Range), pt) {
		return nil
	}

	return []protocol.Location{
		symbol.Location,
	}
}
