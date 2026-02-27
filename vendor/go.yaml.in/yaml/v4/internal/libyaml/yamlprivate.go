//
// Copyright (c) 2011-2019 Canonical Ltd
// Copyright (c) 2006-2010 Kirill Simonov
//
// Permission is hereby granted, free of charge, to any person obtaining a copy of
// this software and associated documentation files (the "Software"), to deal in
// the Software without restriction, including without limitation the rights to
// use, copy, modify, merge, publish, distribute, sublicense, and/or sell copies
// of the Software, and to permit persons to whom the Software is furnished to do
// so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in all
// copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
// SOFTWARE.

package libyaml

const (
	// The size of the input raw buffer.
	input_raw_buffer_size = 512

	// The size of the input buffer.
	// It should be possible to decode the whole raw buffer.
	input_buffer_size = input_raw_buffer_size * 3

	// The size of the output buffer.
	output_buffer_size = 128

	// The size of the output raw buffer.
	// It should be possible to encode the whole output buffer.
	output_raw_buffer_size = (output_buffer_size*2 + 2)

	// The size of other stacks and queues.
	initial_stack_size  = 16
	initial_queue_size  = 16
	initial_string_size = 16
)

// Check if the character at the specified position is an alphabetical
// character, a digit, '_', or '-'.
func isAlpha(b []byte, i int) bool {
	return b[i] >= '0' && b[i] <= '9' || b[i] >= 'A' && b[i] <= 'Z' ||
		b[i] >= 'a' && b[i] <= 'z' || b[i] == '_' || b[i] == '-'
}

// Check if the character at the specified position is a flow indicator as
// defined by spec production [23] c-flow-indicator ::=
// c-collect-entry | c-sequence-start | c-sequence-end |
// c-mapping-start | c-mapping-end
func isFlowIndicator(b []byte, i int) bool {
	return b[i] == '[' || b[i] == ']' ||
		b[i] == '{' || b[i] == '}' || b[i] == ','
}

// Check if the character at the specified position is valid for anchor names
// as defined by spec production [102] ns-anchor-char ::= ns-char -
// c-flow-indicator.
// This includes all printable characters except: CR, LF, BOM, space, tab, '[',
// ']', '{', '}', ','.
// We further limit it to ascii chars only, which is a subset of the spec
// production but is usually what most people expect.
func isAnchorChar(b []byte, i int) bool {
	if isColon(b, i) {
		// [Go] we exclude colons from anchor/alias names.
		//
		// A colon is a valid anchor character according to the YAML 1.2 specification,
		// but it can lead to ambiguity.
		// https://github.com/yaml/go-yaml/issues/109
		//
		// Also, it would have been a breaking change to support it, as go.yaml.in/yaml/v3 ignores it.
		// Supporting it could lead to unexpected behavior.
		return false
	}

	return isPrintable(b, i) &&
		!isLineBreak(b, i) &&
		!isBlank(b, i) &&
		!isBOM(b, i) &&
		!isFlowIndicator(b, i) &&
		isASCII(b, i)
}

// isColon checks whether the character at the specified position is a colon.
func isColon(b []byte, i int) bool {
	return b[i] == ':'
}

// Check if the character at the specified position is a digit.
func isDigit(b []byte, i int) bool {
	return b[i] >= '0' && b[i] <= '9'
}

// Get the value of a digit.
func asDigit(b []byte, i int) int {
	return int(b[i]) - '0'
}

// Check if the character at the specified position is a hex-digit.
func isHex(b []byte, i int) bool {
	return b[i] >= '0' && b[i] <= '9' || b[i] >= 'A' && b[i] <= 'F' ||
		b[i] >= 'a' && b[i] <= 'f'
}

// Get the value of a hex-digit.
func asHex(b []byte, i int) int {
	bi := b[i]
	if bi >= 'A' && bi <= 'F' {
		return int(bi) - 'A' + 10
	}
	if bi >= 'a' && bi <= 'f' {
		return int(bi) - 'a' + 10
	}
	return int(bi) - '0'
}

// Check if the character is ASCII.
func isASCII(b []byte, i int) bool {
	return b[i] <= 0x7F
}

// Check if the character at the start of the buffer can be printed unescaped.
func isPrintable(b []byte, i int) bool {
	return ((b[i] == 0x0A) || // . == #x0A
		(b[i] >= 0x20 && b[i] <= 0x7E) || // #x20 <= . <= #x7E
		(b[i] == 0xC2 && b[i+1] >= 0xA0) || // #0xA0 <= . <= #xD7FF
		(b[i] > 0xC2 && b[i] < 0xED) ||
		(b[i] == 0xED && b[i+1] < 0xA0) ||
		(b[i] == 0xEE) ||
		(b[i] == 0xEF && // #xE000 <= . <= #xFFFD
			!(b[i+1] == 0xBB && b[i+2] == 0xBF) && // && . != #xFEFF
			!(b[i+1] == 0xBF && (b[i+2] == 0xBE || b[i+2] == 0xBF))))
}

// Check if the character at the specified position is NUL.
func isZeroChar(b []byte, i int) bool {
	return b[i] == 0x00
}

// Check if the beginning of the buffer is a BOM.
func isBOM(b []byte, i int) bool {
	return b[0] == 0xEF && b[1] == 0xBB && b[2] == 0xBF
}

// Check if the character at the specified position is space.
func isSpace(b []byte, i int) bool {
	return b[i] == ' '
}

// Check if the character at the specified position is tab.
func isTab(b []byte, i int) bool {
	return b[i] == '\t'
}

// Check if the character at the specified position is blank (space or tab).
func isBlank(b []byte, i int) bool {
	// return isSpace(b, i) || isTab(b, i)
	return b[i] == ' ' || b[i] == '\t'
}

// Check if the character at the specified position is a line break.
func isLineBreak(b []byte, i int) bool {
	return (b[i] == '\r' || // CR (#xD)
		b[i] == '\n' || // LF (#xA)
		b[i] == 0xC2 && b[i+1] == 0x85 || // NEL (#x85)
		b[i] == 0xE2 && b[i+1] == 0x80 && b[i+2] == 0xA8 || // LS (#x2028)
		b[i] == 0xE2 && b[i+1] == 0x80 && b[i+2] == 0xA9) // PS (#x2029)
}

func isCRLF(b []byte, i int) bool {
	return b[i] == '\r' && b[i+1] == '\n'
}

// Check if the character is a line break or NUL.
func isBreakOrZero(b []byte, i int) bool {
	// return isLineBreak(b, i) || isZeroChar(b, i)
	return (
	// isBreak:
	b[i] == '\r' || // CR (#xD)
		b[i] == '\n' || // LF (#xA)
		b[i] == 0xC2 && b[i+1] == 0x85 || // NEL (#x85)
		b[i] == 0xE2 && b[i+1] == 0x80 && b[i+2] == 0xA8 || // LS (#x2028)
		b[i] == 0xE2 && b[i+1] == 0x80 && b[i+2] == 0xA9 || // PS (#x2029)
		// isZeroChar:
		b[i] == 0)
}

// Check if the character is a line break, space, or NUL.
func isSpaceOrZero(b []byte, i int) bool {
	// return isSpace(b, i) || isBreakOrZero(b, i)
	return (
	// isSpace:
	b[i] == ' ' ||
		// isBreakOrZero:
		b[i] == '\r' || // CR (#xD)
		b[i] == '\n' || // LF (#xA)
		b[i] == 0xC2 && b[i+1] == 0x85 || // NEL (#x85)
		b[i] == 0xE2 && b[i+1] == 0x80 && b[i+2] == 0xA8 || // LS (#x2028)
		b[i] == 0xE2 && b[i+1] == 0x80 && b[i+2] == 0xA9 || // PS (#x2029)
		b[i] == 0)
}

// Check if the character is a line break, space, tab, or NUL.
func isBlankOrZero(b []byte, i int) bool {
	// return isBlank(b, i) || isBreakOrZero(b, i)
	return (
	// isBlank:
	b[i] == ' ' || b[i] == '\t' ||
		// isBreakOrZero:
		b[i] == '\r' || // CR (#xD)
		b[i] == '\n' || // LF (#xA)
		b[i] == 0xC2 && b[i+1] == 0x85 || // NEL (#x85)
		b[i] == 0xE2 && b[i+1] == 0x80 && b[i+2] == 0xA8 || // LS (#x2028)
		b[i] == 0xE2 && b[i+1] == 0x80 && b[i+2] == 0xA9 || // PS (#x2029)
		b[i] == 0)
}

// Determine the width of the character.
func width(b byte) int {
	// Don't replace these by a switch without first
	// confirming that it is being inlined.
	if b&0x80 == 0x00 {
		return 1
	}
	if b&0xE0 == 0xC0 {
		return 2
	}
	if b&0xF0 == 0xE0 {
		return 3
	}
	if b&0xF8 == 0xF0 {
		return 4
	}
	return 0
}
