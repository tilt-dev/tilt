package server

import (
	"context"

	"go.lsp.dev/protocol"
)

func (s *Server) DidOpen(ctx context.Context, params *protocol.DidOpenTextDocumentParams) (err error) {
	uri := params.TextDocument.URI
	contents := []byte(params.TextDocument.Text)
	_, err = s.docs.Write(ctx, uri, contents)
	return err
}

func (s *Server) DidChange(ctx context.Context, params *protocol.DidChangeTextDocumentParams) (err error) {
	if len(params.ContentChanges) == 0 {
		return nil
	}

	uri := params.TextDocument.URI
	contents := []byte(params.ContentChanges[0].Text)
	diags, err := s.docs.Write(ctx, uri, contents)
	_ = s.publishDiagnostics(ctx, params.TextDocument, diags)
	return err
}

func (s *Server) DidSave(ctx context.Context, params *protocol.DidSaveTextDocumentParams) (err error) {
	uri := params.TextDocument.URI
	contents := []byte(params.Text)
	_, err = s.docs.Write(ctx, uri, contents)
	return err
}

func (s *Server) DidClose(_ context.Context, params *protocol.DidCloseTextDocumentParams) (err error) {
	s.docs.Remove(params.TextDocument.URI)
	return nil
}

func (s *Server) publishDiagnostics(ctx context.Context, textDoc protocol.VersionedTextDocumentIdentifier, diags []protocol.Diagnostic) error {
	if diags == nil {
		diags = []protocol.Diagnostic{}
	}
	return s.notifier.PublishDiagnostics(ctx, &protocol.PublishDiagnosticsParams{
		URI:         textDoc.URI,
		Version:     uint32(textDoc.Version),
		Diagnostics: diags,
	})
}
