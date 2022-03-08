package server

import (
	"context"

	"go.lsp.dev/protocol"

	"github.com/tilt-dev/starlark-lsp/pkg/query"
)

func (s *Server) DocumentSymbol(ctx context.Context,
	params *protocol.DocumentSymbolParams) ([]interface{}, error) {

	doc, err := s.docs.Read(params.TextDocument.URI)
	if err != nil {
		return nil, err
	}
	defer doc.Close()

	symbols := query.DocumentSymbols(doc)
	result := make([]interface{}, len(symbols))
	for i := range symbols {
		result[i] = symbols[i]
	}
	return result, nil
}
