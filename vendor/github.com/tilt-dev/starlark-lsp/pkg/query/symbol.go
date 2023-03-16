package query

import (
	"fmt"

	"go.lsp.dev/protocol"

	sitter "github.com/smacker/go-tree-sitter"
)

// Get all symbols defined at the same level as the given node.
// If before != nil, only include symbols that appear before that node.
func SiblingSymbols(doc DocumentContent, node, before *sitter.Node) []Symbol {
	var symbols []Symbol
	for n := node; n != nil && NodeBefore(n, before); n = n.NextNamedSibling() {
		var symbol Symbol

		switch n.Type() {
		case NodeTypeExpressionStatement:
			symbol = ExtractVariableAssignment(doc, n)
		case NodeTypeFunctionDef:
			sig := ExtractSignature(doc, n)
			symbol = sig.Symbol()
		}

		if symbol.Name != "" {
			symbols = append(symbols, symbol)
		}
	}
	return symbols
}

func ExtractVariableAssignment(doc DocumentContent, n *sitter.Node) Symbol {
	if n.Type() != NodeTypeExpressionStatement {
		panic(fmt.Errorf("invalid node type: %s", n.Type()))
	}

	var symbol Symbol
	assignment := n.NamedChild(0)
	if assignment == nil || assignment.Type() != "assignment" {
		return symbol
	}
	symbol.Name = doc.Content(assignment.ChildByFieldName("left"))
	val := assignment.ChildByFieldName("right")
	t := assignment.ChildByFieldName("type")
	var kind protocol.SymbolKind
	if t != nil {
		kind = pythonTypeToSymbolKind(doc, t)
	} else if val != nil {
		kind = nodeTypeToSymbolKind(val)
	}
	if kind == 0 {
		kind = protocol.SymbolKindVariable
	}
	symbol.Kind = kind
	symbol.Location = protocol.Location{
		Range: NodeRange(n),
		URI:   doc.URI(),
	}

	// Look for possible docstring for the assigned variable
	if n.NextNamedSibling() != nil && n.NextNamedSibling().Type() == NodeTypeExpressionStatement {
		if ch := n.NextNamedSibling().NamedChild(0); ch != nil && ch.Type() == NodeTypeString {
			symbol.Detail = Unquote(doc.Input(), ch)
		}
	}
	return symbol
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
func SymbolsInScope(doc DocumentContent, node *sitter.Node) []Symbol {
	var symbols []Symbol

	appendParameters := func(fnNode *sitter.Node) {
		sig := ExtractSignature(doc, fnNode)
		for _, p := range sig.Params {
			symbols = append(symbols, p.Symbol())
		}
	}

	// While we are in the current scope, only include symbols defined before
	// the provided node.
	before := node
	n := node
	for ; n.Parent() != nil && !IsModuleScope(doc, n); n = n.Parent() {
		// A function definition creates an enclosing scope, where all symbols
		// in the parent scope are visible. After that point, don't specify a
		// before node.
		if n.Type() == NodeTypeFunctionDef {
			before = nil
			appendParameters(n)
		}

		symbols = append(symbols, SiblingSymbols(doc, n.Parent().NamedChild(0), before)...)
	}
	// Append parameters of parent function that's in module scope
	if n.Type() == NodeTypeFunctionDef {
		appendParameters(n)
	}
	return symbols
}

// DocumentSymbols returns all symbols with document-wide visibility.
func DocumentSymbols(doc DocumentContent) []Symbol {
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

type Symbol struct {
	Name           string
	Detail         string
	Kind           protocol.SymbolKind
	Tags           []protocol.SymbolTag
	Location       protocol.Location
	SelectionRange protocol.Range
	Children       []Symbol
}

// builtins (e.g., `False`, `k8s_resource`) have no location
func (s Symbol) HasLocation() bool {
	return s.Location.URI != ""
}
