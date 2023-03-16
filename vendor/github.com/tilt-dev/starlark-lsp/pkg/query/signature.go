package query

import (
	"fmt"
	"strings"

	"go.lsp.dev/uri"

	"go.lsp.dev/protocol"

	sitter "github.com/smacker/go-tree-sitter"

	"github.com/tilt-dev/starlark-lsp/pkg/docstring"
)

// Functions finds all function definitions that are direct children of the provided sitter.Node.
func Functions(doc DocumentContent, node *sitter.Node) map[string]Signature {
	signatures := make(map[string]Signature)

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
		sig := ExtractSignature(doc, n)
		signatures[sig.Name] = sig
	}

	return signatures
}

// Function finds a function definition for the given function name that is a direct child of the provided sitter.Node.
func Function(doc DocumentContent, node *sitter.Node, fnName string) (Signature, bool) {
	for n := node.NamedChild(0); n != nil; n = n.NextNamedSibling() {
		if n.Type() != NodeTypeFunctionDef {
			continue
		}
		curFuncName := doc.Content(n.ChildByFieldName(FieldName))
		if curFuncName == fnName {
			return ExtractSignature(doc, n), true
		}
	}
	return Signature{}, false
}

type Signature struct {
	Name       string
	Params     []Parameter
	ReturnType string
	Docs       docstring.Parsed
	docURI     uri.URI
	Range      protocol.Range
}

func (s Signature) SignatureInfo() protocol.SignatureInformation {
	params := make([]protocol.ParameterInformation, len(s.Params))
	for i, param := range s.Params {
		params[i] = param.ParameterInfo(s.Docs)
	}
	sigInfo := protocol.SignatureInformation{
		Label:      s.Label(),
		Parameters: params,
	}
	if s.Docs.Description != "" {
		sigInfo.Documentation = protocol.MarkupContent{
			Kind:  protocol.PlainText,
			Value: s.Docs.Description,
		}
	}

	return sigInfo
}

// Label produces a human-readable Label for a function signature.
//
// It's modeled to behave similarly to VSCode Python signature labels.
func (s Signature) Label() string {
	var sb strings.Builder
	sb.WriteRune('(')
	for i := range s.Params {
		sb.WriteString(s.Params[i].Content)
		if i != len(s.Params)-1 {
			sb.WriteString(", ")
		}
	}
	sb.WriteString(")")
	if s.ReturnType != "" {
		sb.WriteString(" -> ")
		sb.WriteString(s.ReturnType)
	}
	return sb.String()
}

func (s Signature) Symbol() Symbol {
	argsList := []string{}
	returns := s.Docs.Returns()
	for _, arg := range s.Docs.Args() {
		argsList = append(argsList, fmt.Sprintf("%s: %s", arg.Name, arg.Desc))
	}
	argsFormatted := strings.Join(argsList, "\\\n")
	detail := s.Docs.Description
	if len(argsFormatted) > 0 {
		detail += fmt.Sprintf("\n## Parameters\n%s", argsFormatted)
	}
	if len(returns) > 0 {
		detail += fmt.Sprintf("\n## Returns\n%s", returns)
	}
	return Symbol{
		Name:   s.Name,
		Kind:   protocol.SymbolKindFunction,
		Detail: detail,
		Location: protocol.Location{
			URI:   s.docURI,
			Range: s.Range,
		},
	}
}

func ExtractSignature(doc DocumentContent, n *sitter.Node) Signature {
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

	return Signature{
		Name:       fnName,
		Params:     params,
		ReturnType: returnType,
		Docs:       fnDocs,
		Range:      NodeRange(n),
		docURI:     doc.URI(),
	}
}

func extractDocstring(doc DocumentContent, n *sitter.Node) docstring.Parsed {
	if n.Type() != NodeTypeBlock {
		panic(fmt.Errorf("invalid node type: %s", n.Type()))
	}

	if exprNode := n.NamedChild(0); exprNode != nil && exprNode.Type() == NodeTypeExpressionStatement {
		if docStringNode := exprNode.NamedChild(0); docStringNode != nil && docStringNode.Type() == NodeTypeString {
			return docstring.Parse(Unquote(doc.Input(), docStringNode))
		}
	}

	// we don't return any sort of bool about success because even if there's
	// a string in the right place in the syntax tree, it might not even be a
	// valid docstring, so this is all on a best effort basis
	return docstring.Parsed{}
}
