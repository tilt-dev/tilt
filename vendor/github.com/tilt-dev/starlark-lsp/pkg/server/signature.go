package server

import (
	"context"

	"go.lsp.dev/protocol"
	"go.uber.org/zap"
)

func (s *Server) SignatureHelp(ctx context.Context,
	params *protocol.SignatureHelpParams) (*protocol.SignatureHelp, error) {
	logger := protocol.LoggerFromContext(ctx).
		With(textDocumentFields(params.TextDocumentPositionParams)...)

	doc, err := s.docs.Read(params.TextDocument.URI)
	if err != nil {
		return nil, err
	}
	defer doc.Close()

	resp := s.analyzer.SignatureHelp(doc, params.Position)
	if resp != nil && len(resp.Signatures) != 0 {
		logger.With(
			zap.Namespace("signature"),
			zap.String("label", resp.Signatures[0].Label),
		).Debug("found signature candidate")
	} else {
		logger.Debug("no signature found")
	}
	return resp, nil
}
