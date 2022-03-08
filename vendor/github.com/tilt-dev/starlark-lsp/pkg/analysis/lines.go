package analysis

import (
	"sort"

	sitter "github.com/smacker/go-tree-sitter"
	"go.lsp.dev/protocol"
)

// LineOffsets translates between byte offsets and (line, col) file locations.
//
// The latter is used throughout the LSP protocol.
// Tree-sitter has good support for both, but it's sometimes easier to use
// byte offsets and translate back later.
//
// Implementation note: there are arguably more efficient ways to compute/store
// this information (e.g. VSCode uses "prefix sums" internally). If this becomes
// a performance bottleneck, it can be optimized. However, this approach is
// similar to others, such as `rust-analyzer`, so it should be sufficient.
//
// Currently, this doesn't handle UTF-16 properly, which is used in the LSP
// spec. See:
// 	* https://github.com/microsoft/language-server-protocol/issues/376
//	* https://github.com/rust-analyzer/rust-analyzer/blob/9b1978a3ed405c2a5ec34703914ec1878b599e14/crates/ide_db/src/line_index.rs
type LineOffsets struct {
	// offsets for the first byte at each line (by index)
	offsets   []uint32
	sourceLen uint32
}

// NewLineOffsets creates a LineOffsets object to convert between byte offsets
// and (line, col) file locations and vice-versa.
func NewLineOffsets(input []byte) LineOffsets {
	var offsets []uint32
	var lineStartOffset uint32 = 0
	for pos, v := range input {
		// common case: newline character
		// special case: files without trailing newline
		if v == '\n' || pos == len(input)-1 {
			offsets = append(offsets, lineStartOffset)
			lineStartOffset = uint32(pos)
		}
	}
	return LineOffsets{
		offsets:   offsets,
		sourceLen: uint32(len(input)),
	}
}

// PositionForOffset returns the file location in LSP protocol format for a given byte offset.
func (l LineOffsets) PositionForOffset(offset uint32) protocol.Position {
	line, col := l.locationForOffset(offset)
	return protocol.Position{
		Line:      line,
		Character: col,
	}
}

// PointForOffset returns the file location in Tree-sitter format for a given byte offset.
func (l LineOffsets) PointForOffset(offset uint32) sitter.Point {
	line, col := l.locationForOffset(offset)
	return sitter.Point{
		Row:    line,
		Column: col,
	}
}

// OffsetForPosition returns the byte offset for a given LSP protocol file location.
func (l LineOffsets) OffsetForPosition(pos protocol.Position) uint32 {
	return l.offsetForLocation(pos.Line, pos.Character)
}

// OffsetForPoint returns the byte offset for a given Tree-sitter file location.
func (l LineOffsets) OffsetForPoint(point sitter.Point) uint32 {
	return l.offsetForLocation(point.Row, point.Column)
}

func (l LineOffsets) locationForOffset(offset uint32) (uint32, uint32) {
	if offset > l.sourceLen {
		offset = l.sourceLen
	}

	line := sort.Search(len(l.offsets), func(i int) bool {
		return l.offsets[i] >= offset
	})

	if line == len(l.offsets) || l.offsets[line] != offset {
		// case 1: offset > last line start, so it's part of the last line
		// case 2: search returns the index where this offset _would_ be
		//         inserted, which is the index of the start of the next line
		//         in the case that the offset isn't for a start of a line
		// in either case, we intentionally overshot and need to use the prior
		// line
		line -= 1
	}

	col := offset - l.offsets[line]
	return uint32(line), col
}

func (l LineOffsets) offsetForLocation(line uint32, col uint32) uint32 {
	if line >= uint32(len(l.offsets)) {
		return l.sourceLen
	}

	offset := l.offsets[line] + col
	if offset >= l.sourceLen {
		return l.sourceLen
	}

	return offset
}
