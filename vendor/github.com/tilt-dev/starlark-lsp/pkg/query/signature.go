package query

import (
	"fmt"
	"strings"

	"go.lsp.dev/protocol"

	sitter "github.com/smacker/go-tree-sitter"

	"github.com/tilt-dev/starlark-lsp/pkg/docstring"
	"github.com/tilt-dev/starlark-lsp/pkg/document"
)

// Functions finds all function definitions that are direct children of the provided sitter.Node.
func Functions(doc document.Document, node *sitter.Node) map[string]protocol.SignatureInformation {
	signatures := make(map[string]protocol.SignatureInformation)

	// N.B. we don't use a query here for a couple reasons:
	// 	(1) Tree-sitter doesn't support bounding the depth, and we only want
	//		direct descendants (to avoid matching on functions in nested scopes)
	//		See https://github.com/tree-sitter/tree-sitter/issues/1212.
	//	(2) function_definition nodes have named fields for what we care about,
	//		which makes it easy to get the data without using a query to help
	//		massage/standardize it (for example, we do this for params since
	//		there are multiple type of param values)
	for n := node.NamedChild(0); n != nil; n = n.NextNamedSibling() {
		if n.Type() != NodeTypeFunctionDef {
			continue
		}
		fnName, sig := extractSignatureInformation(doc, n)
		signatures[fnName] = sig
	}

	return signatures
}

// Function finds a function definition for the given function name that is a direct child of the provided sitter.Node.
func Function(doc document.Document, node *sitter.Node, fnName string) (protocol.SignatureInformation, bool) {
	for n := node.NamedChild(0); n != nil; n = n.NextNamedSibling() {
		if n.Type() != NodeTypeFunctionDef {
			continue
		}
		curFuncName := doc.Content(n.ChildByFieldName(FieldName))
		if curFuncName == fnName {
			_, sig := extractSignatureInformation(doc, n)
			return sig, true
		}
	}
	return protocol.SignatureInformation{}, false
}

func extractSignatureInformation(doc document.Document, n *sitter.Node) (string, protocol.SignatureInformation) {
	if n.Type() != NodeTypeFunctionDef {
		panic(fmt.Errorf("invalid node type: %s", n.Type()))
	}

	fnName := doc.Content(n.ChildByFieldName(FieldName))
	fnDocs := extractDocstring(doc, n.ChildByFieldName(FieldBody))

	// params might be empty but a node for `()` will still exist
	params := extractParameters(doc, fnDocs, n.ChildByFieldName(FieldParameters))
	// unlike name + params, returnType is optional
	var returnType string
	if rtNode := n.ChildByFieldName(FieldReturnType); rtNode != nil {
		returnType = doc.Content(rtNode)
	}

	sig := protocol.SignatureInformation{
		Label:      signatureLabel(params, returnType),
		Parameters: params,
	}

	if fnDocs.Description != "" {
		sig.Documentation = protocol.MarkupContent{
			Kind:  protocol.Markdown,
			Value: fnDocs.Description,
		}
	}

	return fnName, sig
}

// signatureLabel produces a human-readable label for a function signature.
//
// It's modeled to behave similarly to VSCode Python signature labels.
func signatureLabel(params []protocol.ParameterInformation, returnType string) string {
	if returnType == "" {
		returnType = "None"
	}

	var sb strings.Builder
	sb.WriteRune('(')
	for i := range params {
		sb.WriteString(params[i].Label)
		if i != len(params)-1 {
			sb.WriteString(", ")
		}
	}
	sb.WriteString(") -> ")
	sb.WriteString(returnType)
	return sb.String()
}

func extractDocstring(doc document.Document, n *sitter.Node) docstring.Parsed {
	if n.Type() != NodeTypeBlock {
		panic(fmt.Errorf("invalid node type: %s", n.Type()))
	}

	if exprNode := n.NamedChild(0); exprNode != nil && exprNode.Type() == NodeTypeExpressionStatement {
		if docStringNode := exprNode.NamedChild(0); docStringNode != nil && docStringNode.Type() == NodeTypeString {
			// TODO(milas): need to do nested quote un-escaping (generally
			// 	docstrings use triple-quoted strings so this isn't a huge
			// 	issue at least)
			rawDocString := doc.Content(docStringNode)
			// this is the raw source, so it will be wrapped with with """ / ''' / " / '
			// (technically this could trim off too much but not worth the
			// headache to deal with valid leading/trailing quotes)
			rawDocString = strings.Trim(rawDocString, `"'`)
			return docstring.Parse(rawDocString)
		}
	}

	// we don't return any sort of bool about success because even if there's
	// a string in the right place in the syntax tree, it might not even be a
	// valid docstring, so this is all on a best effort basis
	return docstring.Parsed{}
}
