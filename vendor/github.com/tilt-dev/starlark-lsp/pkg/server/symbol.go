package server

import (
	"context"

	"github.com/tilt-dev/starlark-lsp/pkg/query"

	"go.lsp.dev/protocol"
)

func toDocumentSymbol(s query.Symbol) protocol.DocumentSymbol {
	var children []protocol.DocumentSymbol
	for _, c := range s.Children {
		children = append(children, toDocumentSymbol(c))
	}
	return protocol.DocumentSymbol{
		Name:     s.Name,
		Detail:   s.Detail,
		Kind:     s.Kind,
		Tags:     s.Tags,
		Range:    s.Location.Range,
		Children: children,
	}
}

func (s *Server) DocumentSymbol(ctx context.Context,
	params *protocol.DocumentSymbolParams) ([]interface{}, error) {

	doc, err := s.docs.Read(ctx, params.TextDocument.URI)
	if err != nil {
		return nil, err
	}
	defer doc.Close()

	symbols := doc.Symbols()
	result := make([]interface{}, len(symbols))
	for i := range symbols {
		result[i] = toDocumentSymbol(symbols[i])
	}
	return result, nil
}
