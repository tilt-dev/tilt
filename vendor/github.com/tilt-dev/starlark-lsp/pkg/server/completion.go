package server

import (
	"context"

	"go.lsp.dev/protocol"
)

func (s *Server) Completion(ctx context.Context, params *protocol.CompletionParams) (*protocol.CompletionList, error) {
	doc, err := s.docs.Read(params.TextDocument.URI)
	if err != nil {
		return nil, err
	}
	defer doc.Close()

	logger := protocol.LoggerFromContext(ctx).
		With(textDocumentFields(params.TextDocumentPositionParams)...)
	logger.Debug("completion")

	result := s.analyzer.Completion(doc, params.Position)

	return result, nil
}
