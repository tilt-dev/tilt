package query

import (
	sitter "github.com/smacker/go-tree-sitter"
	"go.lsp.dev/protocol"
)

// PositionToPoint converts an LSP protocol file location to a Tree-sitter file location.
func PositionToPoint(pos protocol.Position) sitter.Point {
	return sitter.Point{
		Row:    pos.Line,
		Column: pos.Character,
	}
}

// pointToPosition converts a Tree-sitter file location to an LSP protocol file location.
func pointToPosition(point sitter.Point) protocol.Position {
	return protocol.Position{
		Line:      point.Row,
		Character: point.Column,
	}
}

func NodeRange(node *sitter.Node) protocol.Range {
	return protocol.Range{
		Start: pointToPosition(node.StartPoint()),
		End:   pointToPosition(node.EndPoint()),
	}
}

func NodesRange(nodes []*sitter.Node) protocol.Range {
	var start, end sitter.Point
	for i, n := range nodes {
		if i == 0 || PointBefore(start, n.StartPoint()) {
			start = n.StartPoint()
		}
		if i == 0 || PointAfter(end, n.EndPoint()) {
			end = n.EndPoint()
		}
	}
	return protocol.Range{
		Start: pointToPosition(start),
		End:   pointToPosition(end),
	}
}

func PointCmp(a, b sitter.Point) int {
	if a.Row < b.Row {
		return -1
	}

	if a.Row > b.Row {
		return 1
	}

	if a.Column < b.Column {
		return -1
	}

	if a.Column > b.Column {
		return 1
	}

	return 0
}

func PointBeforeOrEqual(a, b sitter.Point) bool {
	return PointCmp(a, b) <= 0
}

func PointBefore(a, b sitter.Point) bool {
	return PointCmp(a, b) < 0
}

func PointAfterOrEqual(a, b sitter.Point) bool {
	return PointCmp(a, b) >= 0
}

func PointAfter(a, b sitter.Point) bool {
	return PointCmp(a, b) > 0
}

func PointInside(a sitter.Point, b sitter.Range) bool {
	return PointAfter(a, b.StartPoint) && PointBeforeOrEqual(a, b.EndPoint)
}

func PointCovered(a sitter.Point, b *sitter.Node) bool {
	return PointInside(a, sitter.Range{StartPoint: b.StartPoint(), EndPoint: b.EndPoint()})
}

func NodeBefore(a, b *sitter.Node) bool {
	return a != nil && (b == nil || PointBefore(a.StartPoint(), b.StartPoint()))
}

// NamedNodeAtPosition returns the most granular named descendant at a position.
func NamedNodeAtPosition(doc DocumentContent, pos protocol.Position) (*sitter.Node, bool) {
	return NamedNodeAtPoint(doc, PositionToPoint(pos))
}

func NamedNodeAtPoint(doc DocumentContent, pt sitter.Point) (*sitter.Node, bool) {
	if doc.Tree() == nil {
		return nil, false
	}
	node := doc.Tree().RootNode().NamedDescendantForPointRange(pt, pt)
	if node != nil {
		return node, true
	}
	return nil, false
}

func ChildNodeAtPoint(pt sitter.Point, node *sitter.Node) (*sitter.Node, bool) {
	count := int(node.NamedChildCount())
	for i := 0; i < count; i++ {
		child := node.NamedChild(i)
		if PointBeforeOrEqual(child.StartPoint(), pt) && PointBeforeOrEqual(pt, child.EndPoint()) {
			return ChildNodeAtPoint(pt, child)
		}
	}
	return node, true
}

// NodeAtPosition returns the node (named or unnamed) with the smallest
// start/end range that covers the given position.
func NodeAtPosition(doc DocumentContent, pos protocol.Position) (*sitter.Node, bool) {
	return NodeAtPoint(doc, PositionToPoint(pos))
}

func NodeAtPoint(doc DocumentContent, pt sitter.Point) (*sitter.Node, bool) {
	namedNode, ok := NamedNodeAtPoint(doc, pt)
	if !ok {
		return nil, false
	}
	return ChildNodeAtPoint(pt, namedNode)
}
