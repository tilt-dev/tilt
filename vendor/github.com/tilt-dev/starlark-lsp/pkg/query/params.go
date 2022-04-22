package query

import (
	"fmt"

	"go.lsp.dev/uri"

	"go.lsp.dev/protocol"

	sitter "github.com/smacker/go-tree-sitter"

	"github.com/tilt-dev/starlark-lsp/pkg/docstring"
)

// FunctionParameters extracts parameters from a function definition and
// supports a mixture of positional parameters, default value parameters,
// typed parameters*, and typed default value parameters*.
//
// * These are not valid Starlark, but we support them to enable using Python
//   type-stub files for improved editor experience.
const FunctionParameters = `
(parameters ([
    (identifier) @name
    (_ .
        (identifier) @name
        type: (type)? @type
        value: (expression)? @value)
]) @param)
`

type Parameter struct {
	Name         string
	TypeHint     string
	DefaultValue string
	Content      string
	DocURI       uri.URI
	Location     protocol.Location
}

func (p Parameter) ParameterInfo(fnDocs docstring.Parsed) protocol.ParameterInformation {
	// TODO(milas): revisit labels - with type hints this can make signatures
	// 	really long; it might make sense to only include param name and default
	// 	value (if any)
	pi := protocol.ParameterInformation{Label: p.Content}

	var docContent string
	for _, fieldsBlock := range fnDocs.Fields {
		if fieldsBlock.Title != "Args" {
			continue
		}
		for _, f := range fieldsBlock.Fields {
			if f.Name == p.Name {
				docContent = f.Desc
			}
		}
	}

	if docContent != "" {
		pi.Documentation = protocol.MarkupContent{
			Kind:  protocol.PlainText,
			Value: docContent,
		}
	}

	return pi
}

func (p Parameter) Symbol() Symbol {
	return Symbol{
		Name:     p.Name,
		Kind:     protocol.SymbolKindVariable,
		Detail:   p.Content,
		Location: p.Location,
	}
}

func extractParameters(doc DocumentContent, fnDocs docstring.Parsed,
	node *sitter.Node) []Parameter {
	if node.Type() != NodeTypeParameters {
		// A query is used here because there's several different node types
		// for parameter values, and the query handles normalization gracefully
		// for us.
		//
		// Technically, the query will execute regardless of passed in node
		// type, but since Tree-sitter doesn't currently support bounding query
		// depth, that could result in finding parameters from funcs in nested
		// scopes if a block was passed, so this ensures that the actual
		// `parameters` node is passed in here.
		//
		// See https://github.com/tree-sitter/tree-sitter/issues/1212
		panic(fmt.Errorf("invalid node type: %v", node.Type()))
	}

	var params []Parameter
	Query(node, FunctionParameters, func(q *sitter.Query, match *sitter.QueryMatch) bool {
		param := Parameter{
			DocURI: doc.URI(),
		}

		for _, c := range match.Captures {
			content := doc.Content(c.Node)
			switch q.CaptureNameForId(c.Index) {
			case "name":
				param.Name = content
			case "type":
				param.TypeHint = content
			case "value":
				param.DefaultValue = content
			case "param":
				param.Content = content
				param.Location = NodeLocation(c.Node, param.DocURI)
			}
		}

		params = append(params, param)
		return true
	})
	return params
}
