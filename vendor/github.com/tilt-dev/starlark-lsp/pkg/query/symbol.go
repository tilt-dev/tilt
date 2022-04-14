package query

import (
	"strings"

	"go.lsp.dev/protocol"

	sitter "github.com/smacker/go-tree-sitter"
)

// Get all symbols defined at the same level as the given node.
// If before != nil, only include symbols that appear before that node.
func SiblingSymbols(doc DocumentContent, node, before *sitter.Node) []protocol.DocumentSymbol {
	var symbols []protocol.DocumentSymbol
	for n := node; n != nil && NodeBefore(n, before); n = n.NextNamedSibling() {
		var symbol protocol.DocumentSymbol

		if n.Type() == NodeTypeExpressionStatement {
			assignment := n.NamedChild(0)
			if assignment == nil || assignment.Type() != "assignment" {
				continue
			}
			symbol.Name = doc.Content(assignment.ChildByFieldName("left"))
			val := assignment.ChildByFieldName("right")
			var kind protocol.SymbolKind
			if val == nil {
				// python variable assignment without an initial value
				// (https://peps.python.org/pep-0526/); just assume a variable
				kind = 0
			} else {
				kind = nodeTypeToSymbolKind(val)
			}
			if kind == 0 {
				kind = protocol.SymbolKindVariable
			}
			symbol.Kind = kind
			symbol.Range = NodeRange(n)
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
			symbol.Range = NodeRange(n)
		}

		if symbol.Name != "" {
			symbols = append(symbols, symbol)
		}
	}
	return symbols
}

// A node is in the scope of the top level module if there are no function
// definitions in the ancestry of the node.
func IsModuleScope(doc DocumentContent, node *sitter.Node) bool {
	for n := node.Parent(); n != nil; n = n.Parent() {
		if n.Type() == NodeTypeFunctionDef {
			return false
		}
	}
	return true
}

// Get all symbols defined in scopes at or above the level of the given node,
// excluding symbols from the top-level module (document symbols).
func SymbolsInScope(doc DocumentContent, node *sitter.Node) []protocol.DocumentSymbol {
	var symbols []protocol.DocumentSymbol
	// While we are in the current scope, only include symbols defined before
	// the provided node.
	before := node
	for n := node; n.Parent() != nil && !IsModuleScope(doc, n); n = n.Parent() {
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
func DocumentSymbols(doc DocumentContent) []protocol.DocumentSymbol {
	return SiblingSymbols(doc, doc.Tree().RootNode().NamedChild(0), nil)
}

// Returns only the symbols that occur before the node given if any, otherwise return all symbols.
func SymbolsBefore(symbols []protocol.DocumentSymbol, before *sitter.Node) []protocol.DocumentSymbol {
	if before == nil {
		return symbols
	}
	result := []protocol.DocumentSymbol{}
	for _, sym := range symbols {
		symStart := PositionToPoint(sym.Range.Start)
		if PointBefore(symStart, before.StartPoint()) {
			result = append(result, sym)
		}
	}
	return result
}
