package server

import (
	"context"

	"go.lsp.dev/protocol"
)

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
		result[i] = symbols[i]
	}
	return result, nil
}
