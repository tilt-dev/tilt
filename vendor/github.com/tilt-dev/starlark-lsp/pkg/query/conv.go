package query

import (
	sitter "github.com/smacker/go-tree-sitter"
	"go.lsp.dev/protocol"
)

func nodeTypeToSymbolKind(n *sitter.Node) protocol.SymbolKind {
	switch n.Type() {
	case "true":
		return protocol.SymbolKindBoolean
	case "false":
		return protocol.SymbolKindBoolean
	case "list":
		return protocol.SymbolKindArray
	case "dictionary":
		return protocol.SymbolKindObject
	case "integer":
		return protocol.SymbolKindNumber
	case "float":
		return protocol.SymbolKindNumber
	case "none":
		return protocol.SymbolKindNull
	case "string":
		return protocol.SymbolKindString
	case "function_definition":
		return protocol.SymbolKindFunction
	}
	return 0
}
