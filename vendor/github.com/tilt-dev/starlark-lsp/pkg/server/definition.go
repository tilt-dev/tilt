package server

import (
	"context"
	"fmt"

	"go.lsp.dev/protocol"
	"go.uber.org/zap"
)

func (s Server) Definition(ctx context.Context, params *protocol.DefinitionParams) (result []protocol.Location, err error) {
	doc, err := s.docs.Read(ctx, params.TextDocument.URI)
	if err != nil {
		return nil, err
	}
	defer doc.Close()

	logger := protocol.LoggerFromContext(ctx).
		With(textDocumentFields(params.TextDocumentPositionParams)...)
	logger.Debug("definition")

	positions, err := s.analyzer.Definition(ctx, doc, params.Position)
	if err != nil {
		logger.With(zap.Namespace("definition")).Error(fmt.Sprintf("error looking up definition: %v", err))
	} else {
		logger.With(zap.Namespace("definition")).Debug(fmt.Sprintf("found definition locations: %v", positions))
	}

	return positions, err
}
