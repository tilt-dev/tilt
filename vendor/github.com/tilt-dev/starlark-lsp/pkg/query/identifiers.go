package query

import (
	sitter "github.com/smacker/go-tree-sitter"
)

func ExtractIdentifiers(doc DocumentContent, nodes []*sitter.Node, limit *sitter.Point) []string {
	identifiers := []string{}

	for i, node := range nodes {
		switch node.Type() {
		case ".":
			// if first node is a '.' then append as a '.' to indicate an
			// attribute expression attached to some other expression
			if i == 0 {
				identifiers = append(identifiers, ".")
			}
			// if last node is a '.' then append an empty identifier for attribute matching
			if i == len(nodes)-1 {
				identifiers = append(identifiers, "")
			}

		case NodeTypeIdentifier:
			identifiers = append(identifiers, doc.Content(node))

		default:
			// extract identifiers from the subtree using the query
			Query(node, Identifiers, func(q *sitter.Query, match *sitter.QueryMatch) bool {
				for _, c := range match.Captures {
					switch q.CaptureNameForId(c.Index) {
					case "id":
						if limit != nil && PointAfter(c.Node.StartPoint(), *limit) {
							identifiers = append(identifiers, "")
						} else {
							identifiers = append(identifiers, doc.Content(c.Node))
						}
					case "trailing-dot":
						identifiers = append(identifiers, "")
					case "module":
						if c.Node.ChildCount() == 0 {
							identifiers = append(identifiers, "")
						}
					}
				}
				return true
			})
		}
	}

	if len(identifiers) == 0 {
		return []string{""}
	}

	return identifiers
}
