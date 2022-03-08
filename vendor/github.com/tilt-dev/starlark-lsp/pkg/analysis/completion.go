package analysis

import (
	"fmt"
	"regexp"
	"strings"

	"go.lsp.dev/protocol"
	"go.uber.org/zap"

	sitter "github.com/smacker/go-tree-sitter"

	"github.com/tilt-dev/starlark-lsp/pkg/document"
	"github.com/tilt-dev/starlark-lsp/pkg/query"
)

func SymbolMatching(symbols []protocol.DocumentSymbol, name string) protocol.DocumentSymbol {
	for _, sym := range symbols {
		if sym.Name == name {
			return sym
		}
	}
	return protocol.DocumentSymbol{}
}

func SymbolsStartingWith(symbols []protocol.DocumentSymbol, prefix string) []protocol.DocumentSymbol {
	if prefix == "" {
		return symbols
	}
	result := []protocol.DocumentSymbol{}
	for _, sym := range symbols {
		if strings.HasPrefix(sym.Name, prefix) {
			result = append(result, sym)
		}
	}
	return result
}

func ToCompletionItemKind(k protocol.SymbolKind) protocol.CompletionItemKind {
	switch k {
	case protocol.SymbolKindField:
		return protocol.CompletionItemKindField
	case protocol.SymbolKindFunction:
		return protocol.CompletionItemKindFunction
	case protocol.SymbolKindMethod:
		return protocol.CompletionItemKindMethod
	default:
		return protocol.CompletionItemKindVariable
	}
}

func (a *Analyzer) Completion(doc document.Document, pos protocol.Position) *protocol.CompletionList {
	pt := query.PositionToPoint(pos)
	nodes, ok := a.nodesAtPointForCompletion(doc, pt)
	symbols := []protocol.DocumentSymbol{}

	if ok {
		symbols = a.completeExpression(doc, nodes, pt)
	}

	completionList := &protocol.CompletionList{
		Items: make([]protocol.CompletionItem, len(symbols)),
	}

	names := make([]string, len(symbols))
	for i, sym := range symbols {
		names[i] = sym.Name
		var sortText string
		if strings.HasSuffix(sym.Name, "=") {
			sortText = fmt.Sprintf("0%s", sym.Name)
		} else {
			sortText = fmt.Sprintf("1%s", sym.Name)
		}
		completionList.Items[i] = protocol.CompletionItem{
			Label:    sym.Name,
			Detail:   sym.Detail,
			Kind:     ToCompletionItemKind(sym.Kind),
			SortText: sortText,
		}
	}

	if len(names) > 0 {
		a.logger.Debug("completion result", zap.Strings("symbols", names))
	}
	return completionList
}

func (a *Analyzer) completeExpression(doc document.Document, nodes []*sitter.Node, pt sitter.Point) []protocol.DocumentSymbol {
	symbols := []protocol.DocumentSymbol{}
	content := ""

	if len(nodes) > 0 {
		nodeAtPoint := nodes[len(nodes)-1]
		symbols = append(symbols, query.SymbolsInScope(doc, nodeAtPoint)...)
		content = doc.ContentRange(sitter.Range{
			StartByte: nodes[0].StartByte(),
			EndByte:   nodes[len(nodes)-1].EndByte(),
		})

		if fnName, args := keywordArgContext(doc, nodeAtPoint, pt); fnName != "" {
			if fn, ok := a.signatureInformation(doc, nodeAtPoint, fnName); ok {
				symbols = append(symbols, a.keywordArgSymbols(fn, args)...)
			}
		}
	} else {
		content = doc.Content(doc.Tree().RootNode())
	}

	symbols = append(symbols, a.builtins.Symbols...)
	identifiers := query.ExtractIdentifiers(doc, nodes, &pt)

	a.logger.Debug("completion attempt",
		zap.String("code", content),
		zap.Strings("nodes", func() []string {
			types := make([]string, len(nodes))
			for i, n := range nodes {
				types[i] = n.Type()
			}
			return types
		}()),
		zap.Strings("identifiers", identifiers),
	)

	for i, id := range identifiers {
		if i < len(identifiers)-1 {
			sym := SymbolMatching(symbols, id)
			symbols = sym.Children
			a.logger.Debug("children",
				zap.String("id", id),
				zap.Strings("names", func() []string {
					names := make([]string, len(symbols))
					for j, s := range symbols {
						names[j] = s.Name
					}
					return names
				}()))
		} else {
			symbols = SymbolsStartingWith(symbols, id)
		}
	}

	return symbols
}

func (a *Analyzer) nodesAtPointForCompletion(doc document.Document, pt sitter.Point) ([]*sitter.Node, bool) {
	node, ok := query.NodeAtPoint(doc, pt)
	if !ok {
		return []*sitter.Node{}, false
	}
	a.logger.Debug("node at point", zap.String("node", node.Type()))
	return a.nodesForCompletion(doc, node, pt)
}

// Zoom in or out from the node to include adjacent attribute expressions, so we can
// complete starting from the top-most attribute expression.
func (a *Analyzer) nodesForCompletion(doc document.Document, node *sitter.Node, pt sitter.Point) ([]*sitter.Node, bool) {
	nodes := []*sitter.Node{}
	switch node.Type() {
	case query.NodeTypeString, query.NodeTypeComment:
		if query.PointCovered(pt, node) {
			// No completion inside a string or comment
			return nodes, false
		}
	case query.NodeTypeModule:
		// Sometimes the top-level module is the most granular node due to
		// location of the point being between children, in this case, advance
		// to the first child node that appears after the point
		if node.NamedChildCount() > 0 {
			for node = node.NamedChild(0); node != nil && query.PointBefore(node.StartPoint(), pt); {
				next := node.NextNamedSibling()
				if next == nil {
					break
				}
				node = next
			}
			return a.nodesForCompletion(doc, node, pt)
		}

	case query.NodeTypeIfStatement,
		query.NodeTypeExpressionStatement,
		query.NodeTypeForStatement,
		query.NodeTypeAssignment:
		if node.NamedChildCount() == 1 {
			return a.nodesForCompletion(doc, node.NamedChild(0), pt)
		}

		for i := 0; i < int(node.NamedChildCount()); i++ {
			child := node.NamedChild(i)
			if query.PointBefore(child.EndPoint(), pt) {
				return a.leafNodesForCompletion(doc, child, pt)
			}
		}

	case query.NodeTypeAttribute, query.NodeTypeIdentifier:
		// If inside an attribute expression, capture the larger expression for
		// completion.
		switch node.Parent().Type() {
		case query.NodeTypeAttribute:
			nodes, _ = a.nodesForCompletion(doc, node.Parent(), pt)
		}

	case query.NodeTypeERROR, query.NodeTypeArgList:
		leafNodes, ok := a.leafNodesForCompletion(doc, node, pt)
		if len(leafNodes) > 0 {
			return leafNodes, ok
		}
		node = node.Child(int(node.ChildCount()) - 1)
	}

	if len(nodes) == 0 {
		nodes = append(nodes, node)
	}
	return nodes, true
}

// Look at all leaf nodes for the node and its previous sibling in a
// flattened slice, in order of appearance. Take all consecutive trailing
// identifiers or '.' as the attribute expression to complete.
func (a *Analyzer) leafNodesForCompletion(doc document.Document, node *sitter.Node, pt sitter.Point) ([]*sitter.Node, bool) {
	leafNodes := []*sitter.Node{}

	if node.PrevNamedSibling() != nil {
		leafNodes = append(leafNodes, query.LeafNodes(node.PrevNamedSibling())...)
	}

	leafNodes = append(leafNodes, query.LeafNodes(node)...)

	// count number of trailing id/'.' nodes, if any
	trailingCount := 0
	leafCount := len(leafNodes)
	for i := 0; i < leafCount && i == trailingCount; i++ {
		switch leafNodes[leafCount-1-i].Type() {
		case query.NodeTypeIdentifier, ".":
			trailingCount++
		}
	}
	nodes := make([]*sitter.Node, trailingCount)
	for j := 0; j < len(nodes); j++ {
		nodes[j] = leafNodes[leafCount-trailingCount+j]
	}

	return nodes, true
}

// TODO: retain parsed function and parameter data so it doesn't need to be
// parsed out of ParameterInformation.Label
var paramName = regexp.MustCompile(`^(\w+)`)

func (a *Analyzer) keywordArgSymbols(fn protocol.SignatureInformation, args callArguments) []protocol.DocumentSymbol {
	symbols := []protocol.DocumentSymbol{}
	for i, param := range fn.Parameters {
		if i < int(args.positional) {
			continue
		}
		label := param.Label
		match := paramName.FindSubmatch([]byte(label))
		if match == nil {
			continue
		}
		kwarg := string(match[1])
		if used := args.keywords[kwarg]; !used {
			symbols = append(symbols, protocol.DocumentSymbol{
				Name:   kwarg + "=",
				Detail: param.Label,
				Kind:   protocol.SymbolKindVariable,
			})
		}
	}
	return symbols
}

func keywordArgContext(doc document.Document, node *sitter.Node, pt sitter.Point) (fnName string, args callArguments) {
	if node.Type() == "=" ||
		query.HasAncestor(node, func(anc *sitter.Node) bool {
			return anc.Type() == query.NodeTypeKeywordArgument
		}) {
		return "", callArguments{}
	}
	return possibleCallInfo(doc, node, pt)
}
