package analysis

import (
	sitter "github.com/smacker/go-tree-sitter"
	"go.lsp.dev/protocol"

	"github.com/tilt-dev/starlark-lsp/pkg/document"
	"github.com/tilt-dev/starlark-lsp/pkg/query"
)

func (a *Analyzer) signatureInformation(doc document.Document, node *sitter.Node, fnName string) (protocol.SignatureInformation, bool) {
	var sig protocol.SignatureInformation
	var found bool

	for n := node; n != nil && !query.IsModuleScope(doc, n); n = n.Parent() {
		sig, found = query.Function(doc, n, fnName)
		if found {
			break
		}
	}

	if !found {
		sig, found = doc.Functions()[fnName]
	}

	if !found {
		sig = a.builtins.Functions[fnName]
	}

	return sig, sig.Label != ""
}

func (a *Analyzer) SignatureHelp(doc document.Document, pos protocol.Position) *protocol.SignatureHelp {
	pt := query.PositionToPoint(pos)
	node, ok := query.NodeAtPoint(doc, pt)
	if !ok {
		return nil
	}

	fnName, args := possibleCallInfo(doc, node, pt)
	if fnName == "" {
		// avoid computing function defs
		return nil
	}

	sig, ok := a.signatureInformation(doc, node, fnName)
	if !ok {
		return nil
	}

	activeParam := uint32(0)

	if args.positional == args.total {
		activeParam = args.positional
	}

	if activeParam > uint32(len(sig.Parameters)-1) {
		activeParam = uint32(len(sig.Parameters) - 1)
	}

	return &protocol.SignatureHelp{
		Signatures:      []protocol.SignatureInformation{sig},
		ActiveParameter: activeParam,
		ActiveSignature: 0,
	}
}

type callArguments struct {
	positional, total uint32
	keywords          map[string]bool
}

// possibleCallInfo attempts to find the name of the function for a
// `call`.
//
// Currently, this supports two cases:
// 	(1) Current node is inside of a `call`
// 	(2) Current node is inside of an ERROR block where first child is an
// 		`identifier`
func possibleCallInfo(doc document.Document, node *sitter.Node, pt sitter.Point) (fnName string, args callArguments) {
	for n := node; n != nil; n = n.Parent() {
		if n.Type() == "call" {
			fnName = doc.Content(n.ChildByFieldName("function"))
			args = possibleActiveParam(doc, n.ChildByFieldName("arguments").Child(0), pt)
			return fnName, args
		} else if n.HasError() {
			// look for `foo(` and assume it's a function call - this could
			// happen if the closing `)` is not (yet) present or if there's
			// something invalid going on within the args, e.g. `foo(x#)`
			possibleCall := n.NamedChild(0)
			if possibleCall != nil && possibleCall.Type() == query.NodeTypeIdentifier {
				possibleParen := possibleCall.NextSibling()
				if possibleParen != nil && !possibleParen.IsNamed() && doc.Content(possibleParen) == "(" {
					fnName = doc.Content(possibleCall)
					args = possibleActiveParam(doc, possibleParen.NextSibling(), pt)
					return fnName, args
				}
			}
		}
	}
	return "", callArguments{}
}

func possibleActiveParam(doc document.Document, node *sitter.Node, pt sitter.Point) callArguments {
	args := callArguments{keywords: make(map[string]bool)}
	for n := node; n != nil; n = n.NextSibling() {
		inRange := query.PointBeforeOrEqual(n.StartPoint(), pt) &&
			query.PointBeforeOrEqual(n.EndPoint(), pt)
		if !inRange {
			break
		}

		if !n.IsNamed() && doc.Content(n) == "," {
			args.total++
			if len(args.keywords) == 0 {
				args.positional++
			}
			continue
		}
		if n.Type() == query.NodeTypeKeywordArgument {
			name := doc.Content(n.ChildByFieldName("name"))
			args.keywords[name] = true
		}
	}
	return args
}
