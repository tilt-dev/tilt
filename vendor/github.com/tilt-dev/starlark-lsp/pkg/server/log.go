package server

import (
	"go.lsp.dev/protocol"
	"go.uber.org/zap"
)

func positionField(pos protocol.Position) zap.Field {
	return zap.Uint32s("pos", []uint32{pos.Line, pos.Character})
}

func uriField(uri protocol.URI) zap.Field {
	return zap.String("uri", string(uri))
}

func textDocumentFields(params protocol.TextDocumentPositionParams) []zap.Field {
	return []zap.Field{
		uriField(params.TextDocument.URI),
		positionField(params.Position),
	}
}
