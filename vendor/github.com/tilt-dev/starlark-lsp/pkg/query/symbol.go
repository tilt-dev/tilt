package query

import (
	"strings"

	"go.lsp.dev/protocol"

	sitter "github.com/smacker/go-tree-sitter"

	"github.com/tilt-dev/starlark-lsp/pkg/document"
)

// Get all symbols defined at the same level as the given node.
// If before != nil, only include symbols that appear before that node.
func SiblingSymbols(doc document.Document, node, before *sitter.Node) []protocol.DocumentSymbol {
	var symbols []protocol.DocumentSymbol
	for n := node; n != nil && NodeBefore(n, before); n = n.NextNamedSibling() {
		var symbol protocol.DocumentSymbol

		if n.Type() == NodeTypeExpressionStatement {
			assignment := n.NamedChild(0)
			if assignment == nil || assignment.Type() != "assignment" {
				continue
			}
			symbol.Name = doc.Content(assignment.ChildByFieldName("left"))
			kind := nodeTypeToSymbolKind(assignment.ChildByFieldName("right"))
			if kind == 0 {
				kind = protocol.SymbolKindVariable
			}
			symbol.Kind = kind
			symbol.Range = protocol.Range{
				Start: PointToPosition(n.StartPoint()),
				End:   PointToPosition(n.EndPoint()),
			}
			// Look for possible docstring for the assigned variable
			if n.NextNamedSibling() != nil && n.NextNamedSibling().Type() == NodeTypeExpressionStatement {
				if ch := n.NextNamedSibling().NamedChild(0); ch != nil && ch.Type() == NodeTypeString {
					symbol.Detail = strings.Trim(doc.Content(ch), `"'`)
				}
			}
		}

		if n.Type() == NodeTypeFunctionDef {
			name, sigInfo := extractSignatureInformation(doc, n)
			symbol.Name = name
			symbol.Kind = protocol.SymbolKindFunction
			symbol.Detail = sigInfo.Label
			symbol.Range = protocol.Range{
				Start: PointToPosition(n.StartPoint()),
				End:   PointToPosition(n.EndPoint()),
			}
		}

		if symbol.Name != "" {
			symbols = append(symbols, symbol)
		}
	}
	return symbols
}

// Get all symbols defined in scopes at or above the level of the given node.
func SymbolsInScope(doc document.Document, node *sitter.Node) []protocol.DocumentSymbol {
	var symbols []protocol.DocumentSymbol
	// While we are in the current scope, only include symbols defined before
	// the provided node.
	before := node
	for n := node; n.Parent() != nil; n = n.Parent() {
		// A function definition creates an enclosing scope, where all symbols
		// in the parent scope are visible. After that point, don't specify a
		// before node.
		if n.Type() == NodeTypeFunctionDef {
			before = nil
		}

		symbols = append(symbols, SiblingSymbols(doc, n.Parent().NamedChild(0), before)...)
	}
	return symbols
}

// DocumentSymbols returns all symbols with document-wide visibility.
func DocumentSymbols(doc document.Document) []protocol.DocumentSymbol {
	return SiblingSymbols(doc, doc.Tree().RootNode().NamedChild(0), nil)
}
