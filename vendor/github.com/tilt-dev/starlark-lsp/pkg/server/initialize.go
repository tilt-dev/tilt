package server

import (
	"context"

	"go.lsp.dev/protocol"
)

func (s *Server) Initialize(ctx context.Context,
	params *protocol.InitializeParams) (result *protocol.InitializeResult, err error) {
	_ = s.notifier.LogMessage(ctx, &protocol.LogMessageParams{
		Message: "Starlark LSP server initialized",
		Type:    protocol.MessageTypeLog,
	})

	s.docs.Initialize(params)
	return &protocol.InitializeResult{
		Capabilities: protocol.ServerCapabilities{
			TextDocumentSync: protocol.TextDocumentSyncOptions{
				Change:    protocol.TextDocumentSyncKindFull,
				OpenClose: true,
				Save: &protocol.SaveOptions{
					IncludeText: true,
				},
			},
			SignatureHelpProvider: &protocol.SignatureHelpOptions{
				TriggerCharacters:   []string{"("},
				RetriggerCharacters: []string{","},
			},
			DocumentSymbolProvider: true,
			CompletionProvider: &protocol.CompletionOptions{
				TriggerCharacters: []string{"."},
			},
			HoverProvider: true,
		},
	}, nil
}
