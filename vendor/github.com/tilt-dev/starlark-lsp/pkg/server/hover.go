package server

import (
	"context"

	"go.lsp.dev/protocol"
)

func (s *Server) Hover(ctx context.Context, params *protocol.HoverParams) (result *protocol.Hover, err error) {
	doc, err := s.docs.Read(ctx, params.TextDocument.URI)
	if err != nil {
		return nil, err
	}
	defer doc.Close()

	logger := protocol.LoggerFromContext(ctx).
		With(textDocumentFields(params.TextDocumentPositionParams)...)
	logger.Debug("completion")

	return s.analyzer.Hover(ctx, doc, params.Position), nil
}
