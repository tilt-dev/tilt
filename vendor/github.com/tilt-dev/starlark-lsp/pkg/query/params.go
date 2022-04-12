package query

import (
	"fmt"

	"go.lsp.dev/protocol"

	sitter "github.com/smacker/go-tree-sitter"

	"github.com/tilt-dev/starlark-lsp/pkg/docstring"
)

type parameter struct {
	name         string
	typeHint     string
	defaultValue string
	content      string
}

func (p parameter) paramInfo(fnDocs docstring.Parsed) protocol.ParameterInformation {
	// TODO(milas): revisit labels - with type hints this can make signatures
	// 	really long; it might make sense to only include param name and default
	// 	value (if any)
	pi := protocol.ParameterInformation{Label: p.content}

	var docContent string
	for _, fieldsBlock := range fnDocs.Fields {
		if fieldsBlock.Title != "Args" {
			continue
		}
		for _, f := range fieldsBlock.Fields {
			if f.Name == p.name {
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

func extractParameters(doc DocumentContent, fnDocs docstring.Parsed,
	node *sitter.Node) []protocol.ParameterInformation {
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

	var params []protocol.ParameterInformation
	Query(node, FunctionParameters, func(q *sitter.Query, match *sitter.QueryMatch) bool {
		var param parameter

		for _, c := range match.Captures {
			content := doc.Content(c.Node)
			switch q.CaptureNameForId(c.Index) {
			case "name":
				param.name = content
			case "type":
				param.typeHint = content
			case "value":
				param.defaultValue = content
			case "param":
				param.content = content
			}
		}

		params = append(params, param.paramInfo(fnDocs))
		return true
	})
	return params
}
