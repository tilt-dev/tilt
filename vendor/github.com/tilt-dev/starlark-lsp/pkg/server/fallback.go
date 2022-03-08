package server

import (
	"context"

	"go.lsp.dev/jsonrpc2"
	"go.lsp.dev/protocol"
)

// FallbackServer is a complete implementation of the protocol.Server interface
// that responds with a "method not found" error for all methods.
//
// LSP provides a rich capabilities system, so servers don't need to implement
// all functionality. However, the protocol.Server interface contains ALL
// possible methods yet is desirable to use because a jsonrpc2.Handler wrapper
// is provided which handles repetitive logic like (un)marshaling.
//
// FallbackServer is intended to be embedded within a real server struct
// implementation, which can then override methods as needed. This is purely
// for convenience and to make the code easier to browse, as the numerous
// stub methods are not intermingled with actual functionality.
type FallbackServer struct{}

var _ protocol.Server = FallbackServer{}

func (f FallbackServer) Initialize(_ context.Context,
	_ *protocol.InitializeParams) (result *protocol.InitializeResult, err error) {
	return nil, jsonrpc2.ErrMethodNotFound
}

func (f FallbackServer) Initialized(_ context.Context, _ *protocol.InitializedParams) (err error) {
	return jsonrpc2.ErrMethodNotFound
}

func (f FallbackServer) Shutdown(_ context.Context) (err error) {
	return jsonrpc2.ErrMethodNotFound
}

func (f FallbackServer) Exit(_ context.Context) (err error) {
	return jsonrpc2.ErrMethodNotFound
}

func (f FallbackServer) WorkDoneProgressCancel(_ context.Context,
	_ *protocol.WorkDoneProgressCancelParams) (err error) {
	return jsonrpc2.ErrMethodNotFound
}

func (f FallbackServer) LogTrace(_ context.Context, _ *protocol.LogTraceParams) (err error) {
	return jsonrpc2.ErrMethodNotFound
}

func (f FallbackServer) SetTrace(_ context.Context, _ *protocol.SetTraceParams) (err error) {
	return jsonrpc2.ErrMethodNotFound
}

func (f FallbackServer) CodeAction(_ context.Context,
	_ *protocol.CodeActionParams) (result []protocol.CodeAction, err error) {
	return nil, jsonrpc2.ErrMethodNotFound
}

func (f FallbackServer) CodeLens(_ context.Context,
	_ *protocol.CodeLensParams) (result []protocol.CodeLens, err error) {
	return nil, jsonrpc2.ErrMethodNotFound
}

func (f FallbackServer) CodeLensResolve(_ context.Context,
	_ *protocol.CodeLens) (result *protocol.CodeLens, err error) {
	return nil, jsonrpc2.ErrMethodNotFound
}

func (f FallbackServer) ColorPresentation(_ context.Context,
	_ *protocol.ColorPresentationParams) (result []protocol.ColorPresentation, err error) {
	return nil, jsonrpc2.ErrMethodNotFound
}

func (f FallbackServer) Completion(_ context.Context,
	_ *protocol.CompletionParams) (result *protocol.CompletionList, err error) {
	return nil, jsonrpc2.ErrMethodNotFound
}

func (f FallbackServer) CompletionResolve(_ context.Context,
	_ *protocol.CompletionItem) (result *protocol.CompletionItem, err error) {
	return nil, jsonrpc2.ErrMethodNotFound
}

func (f FallbackServer) Declaration(_ context.Context,
	_ *protocol.DeclarationParams) (result []protocol.Location, err error) {
	return nil, jsonrpc2.ErrMethodNotFound
}

func (f FallbackServer) Definition(_ context.Context,
	_ *protocol.DefinitionParams) (result []protocol.Location, err error) {
	return nil, jsonrpc2.ErrMethodNotFound
}

func (f FallbackServer) DidChange(_ context.Context, _ *protocol.DidChangeTextDocumentParams) (err error) {
	return jsonrpc2.ErrMethodNotFound
}

func (f FallbackServer) DidChangeConfiguration(_ context.Context,
	_ *protocol.DidChangeConfigurationParams) (err error) {
	return jsonrpc2.ErrMethodNotFound
}

func (f FallbackServer) DidChangeWatchedFiles(_ context.Context,
	_ *protocol.DidChangeWatchedFilesParams) (err error) {
	return jsonrpc2.ErrMethodNotFound
}

func (f FallbackServer) DidChangeWorkspaceFolders(_ context.Context,
	_ *protocol.DidChangeWorkspaceFoldersParams) (err error) {
	return jsonrpc2.ErrMethodNotFound
}

func (f FallbackServer) DidClose(_ context.Context, _ *protocol.DidCloseTextDocumentParams) (err error) {
	return jsonrpc2.ErrMethodNotFound
}

func (f FallbackServer) DidOpen(_ context.Context, _ *protocol.DidOpenTextDocumentParams) (err error) {
	return jsonrpc2.ErrMethodNotFound
}

func (f FallbackServer) DidSave(_ context.Context, _ *protocol.DidSaveTextDocumentParams) (err error) {
	return jsonrpc2.ErrMethodNotFound
}

func (f FallbackServer) DocumentColor(_ context.Context,
	_ *protocol.DocumentColorParams) (result []protocol.ColorInformation, err error) {
	return nil, jsonrpc2.ErrMethodNotFound
}

func (f FallbackServer) DocumentHighlight(_ context.Context,
	_ *protocol.DocumentHighlightParams) (result []protocol.DocumentHighlight, err error) {
	return nil, jsonrpc2.ErrMethodNotFound
}

func (f FallbackServer) DocumentLink(_ context.Context,
	_ *protocol.DocumentLinkParams) (result []protocol.DocumentLink, err error) {
	return nil, jsonrpc2.ErrMethodNotFound
}

func (f FallbackServer) DocumentLinkResolve(_ context.Context,
	_ *protocol.DocumentLink) (result *protocol.DocumentLink, err error) {
	return nil, jsonrpc2.ErrMethodNotFound
}

func (f FallbackServer) DocumentSymbol(_ context.Context,
	_ *protocol.DocumentSymbolParams) (result []interface{}, err error) {
	return nil, jsonrpc2.ErrMethodNotFound
}

func (f FallbackServer) ExecuteCommand(_ context.Context,
	_ *protocol.ExecuteCommandParams) (result interface{}, err error) {
	return nil, jsonrpc2.ErrMethodNotFound
}

func (f FallbackServer) FoldingRanges(_ context.Context,
	_ *protocol.FoldingRangeParams) (result []protocol.FoldingRange, err error) {
	return nil, jsonrpc2.ErrMethodNotFound
}

func (f FallbackServer) Formatting(_ context.Context,
	_ *protocol.DocumentFormattingParams) (result []protocol.TextEdit, err error) {
	return nil, jsonrpc2.ErrMethodNotFound
}

func (f FallbackServer) Hover(_ context.Context,
	_ *protocol.HoverParams) (result *protocol.Hover, err error) {
	return nil, jsonrpc2.ErrMethodNotFound
}

func (f FallbackServer) Implementation(_ context.Context,
	_ *protocol.ImplementationParams) (result []protocol.Location, err error) {
	return nil, jsonrpc2.ErrMethodNotFound
}

func (f FallbackServer) OnTypeFormatting(_ context.Context,
	_ *protocol.DocumentOnTypeFormattingParams) (result []protocol.TextEdit, err error) {
	return nil, jsonrpc2.ErrMethodNotFound
}

func (f FallbackServer) PrepareRename(_ context.Context,
	_ *protocol.PrepareRenameParams) (result *protocol.Range, err error) {
	return nil, jsonrpc2.ErrMethodNotFound
}

func (f FallbackServer) RangeFormatting(_ context.Context,
	_ *protocol.DocumentRangeFormattingParams) (result []protocol.TextEdit, err error) {
	return nil, jsonrpc2.ErrMethodNotFound
}

func (f FallbackServer) References(_ context.Context,
	_ *protocol.ReferenceParams) (result []protocol.Location, err error) {
	return nil, jsonrpc2.ErrMethodNotFound
}

func (f FallbackServer) Rename(_ context.Context,
	_ *protocol.RenameParams) (result *protocol.WorkspaceEdit, err error) {
	return nil, jsonrpc2.ErrMethodNotFound
}

func (f FallbackServer) SignatureHelp(_ context.Context,
	_ *protocol.SignatureHelpParams) (result *protocol.SignatureHelp, err error) {
	return nil, jsonrpc2.ErrMethodNotFound
}

func (f FallbackServer) Symbols(_ context.Context,
	_ *protocol.WorkspaceSymbolParams) (result []protocol.SymbolInformation, err error) {
	return nil, jsonrpc2.ErrMethodNotFound
}

func (f FallbackServer) TypeDefinition(_ context.Context,
	_ *protocol.TypeDefinitionParams) (result []protocol.Location, err error) {
	return nil, jsonrpc2.ErrMethodNotFound
}

func (f FallbackServer) WillSave(_ context.Context, _ *protocol.WillSaveTextDocumentParams) (err error) {
	return jsonrpc2.ErrMethodNotFound
}

func (f FallbackServer) WillSaveWaitUntil(_ context.Context,
	_ *protocol.WillSaveTextDocumentParams) (result []protocol.TextEdit, err error) {
	return nil, jsonrpc2.ErrMethodNotFound
}

func (f FallbackServer) ShowDocument(_ context.Context,
	_ *protocol.ShowDocumentParams) (result *protocol.ShowDocumentResult, err error) {
	return nil, jsonrpc2.ErrMethodNotFound
}

func (f FallbackServer) WillCreateFiles(_ context.Context,
	_ *protocol.CreateFilesParams) (result *protocol.WorkspaceEdit, err error) {
	return nil, jsonrpc2.ErrMethodNotFound
}

func (f FallbackServer) DidCreateFiles(_ context.Context, _ *protocol.CreateFilesParams) (err error) {
	return jsonrpc2.ErrMethodNotFound
}

func (f FallbackServer) WillRenameFiles(_ context.Context,
	_ *protocol.RenameFilesParams) (result *protocol.WorkspaceEdit, err error) {
	return nil, jsonrpc2.ErrMethodNotFound
}

func (f FallbackServer) DidRenameFiles(_ context.Context, _ *protocol.RenameFilesParams) (err error) {
	return jsonrpc2.ErrMethodNotFound
}

func (f FallbackServer) WillDeleteFiles(_ context.Context,
	_ *protocol.DeleteFilesParams) (result *protocol.WorkspaceEdit, err error) {
	return nil, jsonrpc2.ErrMethodNotFound
}

func (f FallbackServer) DidDeleteFiles(_ context.Context, _ *protocol.DeleteFilesParams) (err error) {
	return jsonrpc2.ErrMethodNotFound
}

func (f FallbackServer) CodeLensRefresh(_ context.Context) (err error) {
	return jsonrpc2.ErrMethodNotFound
}

func (f FallbackServer) PrepareCallHierarchy(_ context.Context,
	_ *protocol.CallHierarchyPrepareParams) (result []protocol.CallHierarchyItem, err error) {
	return nil, jsonrpc2.ErrMethodNotFound
}

func (f FallbackServer) IncomingCalls(_ context.Context,
	_ *protocol.CallHierarchyIncomingCallsParams) (result []protocol.CallHierarchyIncomingCall, err error) {
	return nil, jsonrpc2.ErrMethodNotFound
}

func (f FallbackServer) OutgoingCalls(_ context.Context,
	_ *protocol.CallHierarchyOutgoingCallsParams) (result []protocol.CallHierarchyOutgoingCall, err error) {
	return nil, jsonrpc2.ErrMethodNotFound
}

func (f FallbackServer) SemanticTokensFull(_ context.Context,
	_ *protocol.SemanticTokensParams) (result *protocol.SemanticTokens, err error) {
	return nil, jsonrpc2.ErrMethodNotFound
}

func (f FallbackServer) SemanticTokensFullDelta(_ context.Context,
	_ *protocol.SemanticTokensDeltaParams) (result interface{}, err error) {
	return nil, jsonrpc2.ErrMethodNotFound
}

func (f FallbackServer) SemanticTokensRange(_ context.Context,
	_ *protocol.SemanticTokensRangeParams) (result *protocol.SemanticTokens, err error) {
	return nil, jsonrpc2.ErrMethodNotFound
}

func (f FallbackServer) SemanticTokensRefresh(_ context.Context) (err error) {
	return jsonrpc2.ErrMethodNotFound
}

func (f FallbackServer) LinkedEditingRange(_ context.Context,
	_ *protocol.LinkedEditingRangeParams) (result *protocol.LinkedEditingRanges, err error) {
	return nil, jsonrpc2.ErrMethodNotFound
}

func (f FallbackServer) Moniker(_ context.Context,
	_ *protocol.MonikerParams) (result []protocol.Moniker, err error) {
	return nil, jsonrpc2.ErrMethodNotFound
}

func (f FallbackServer) Request(_ context.Context, _ string,
	_ interface{}) (result interface{}, err error) {
	return nil, jsonrpc2.ErrMethodNotFound
}
