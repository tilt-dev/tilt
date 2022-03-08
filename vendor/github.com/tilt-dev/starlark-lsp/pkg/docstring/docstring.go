// Copyright 2019 The LUCI Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Package docstring parses docstrings into more structured representation.
//
// Understands doc strings of the following form.
//
// """Paragraph.
// Perhaps multiline.
//
// Another paragraph.
//   With indentation.
//
// Args:
//   arg1: desc,
//     perhaps multiline, but must be intended.
//   arg2: ...
//
// Returns:
//   Intended free form text.
// """
//
// Extracts all relevant parts of the docstring, deindending them as necessary.
package docstring

import (
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"
)

// Parsed is a parsed docstring.
//
// It is a block of a text (presumably describing how to use a function),
// followed by a parsed arguments list (or equivalent, e.g. list of fields in
// a struct), followed by zero or more "remarks" blocks, which are named
// free-form text blocks. Most common remark block is "Returns", describing what
// the function returns.
type Parsed struct {
	Description string        // deindented function description
	Fields      []FieldsBlock // all found fields blocks, e.g. "Args"
	Remarks     []RemarkBlock // all found remark blocks, e.g. "Returns"
}

// FieldsBlock returns a fields block with the given title or an empty block if
// not found.
func (p *Parsed) FieldsBlock(title string) FieldsBlock {
	for _, b := range p.Fields {
		if b.Title == title {
			return b
		}
	}
	return FieldsBlock{}
}

// RemarkBlock returns a remark block with the given title or an empty block if
// not found.
func (p *Parsed) RemarkBlock(title string) RemarkBlock {
	for _, b := range p.Remarks {
		if b.Title == title {
			return b
		}
	}
	return RemarkBlock{}
}

// Args is an alias for FieldsBlock("Args").Fields.
//
// Returns arguments accepted by a function.
func (p *Parsed) Args() []Field {
	return p.FieldsBlock("Args").Fields
}

// Returns is an alias for RemarkBlock("Returns").Body.
//
// Returns a description of a function's return value.
func (p *Parsed) Returns() string {
	return p.RemarkBlock("Returns").Body
}

// FieldsBlock is a section like "Args: ..." with a bunch of field definitions.
type FieldsBlock struct {
	Title  string  // how this block is titled, e.g. "Args" or "Fields"
	Fields []Field // each defined field
}

// Field represents single "<name>: blah-blah-blah" definition.
type Field struct {
	Name string // name of the field
	Desc string // field's description, "\n" is replaced with " "
}

// RemarkBlock represents things like "Returns:\n blah-blah".
//
// We do not try to parse the body.
type RemarkBlock struct {
	Title string // e.g. "Returns"
	Body  string // deindented  body
}

// Parse parses as much of the docstring as possible.
//
// The expected grammar (loosely, since it is complicated by indentation
// handling):
//
//     Parsed -> Block*
//     Block -> []string | (FieldsBlock | RemarkBlock)*
//     Fields -> ("Args:" | "Field:" | ...) Field+
//     Field -> "  <name>:" []string
//     RemarkBlock -> ("Returns:" | "Note:" | "...") []string
//
// Never fails. May return incomplete or even empty object if the string format
// is unrecognized.
func Parse(doc string) Parsed {
	var out Parsed
	lines := normalizedLines(doc)

	var descLines []string
	for len(lines) > 0 {
		// Read the description until we hit a first "\n<Word>:" line which marks
		// a beginning of either FieldsBlock or RemarkBlock.
		var desc []string
		desc, lines = readUntil(lines, func(prev *string, line string) (stop bool) {
			// Either no previous line at all, or an empty previous line.
			if prev == nil || *prev == "" {
				_, stop = parseBlockTitle(line)
			}
			return
		})
		descLines = append(descLines, trimEmptyLines(desc)...)

		if len(lines) == 0 {
			break
		}

		// This is e.g. "Args" or "Returns".
		title, _ := parseBlockTitle(lines[0])
		lines = lines[1:]

		// "Args" and "Returns" blocks are indented. Read the entire block, i.e.
		// until the indentation returns back to 0.
		var block []string
		block, lines = readUntil(lines, func(_ *string, l string) bool {
			return l != "" && !hasLeadingSpace(l)
		})
		block = trimEmptyLines(deindent(block))

		// Now we can figure out what kind of block this is. Field blocks have all
		// non-indented lines start with field definitions "arg: ...". Remark blocks
		// are free form.
		isFieldsBlock := false
		for _, l := range block {
			if l == "" || hasLeadingSpace(l) {
				continue
			}
			if _, _, ok := parseFieldLine(l); !ok {
				isFieldsBlock = false // found a non-field line, give up
				break
			}
			isFieldsBlock = true // found at least one field
		}

		if isFieldsBlock {
			out.Fields = append(out.Fields, FieldsBlock{
				Title:  title,
				Fields: parseFields(block),
			})
		} else {
			out.Remarks = append(out.Remarks, RemarkBlock{
				Title: title,
				Body:  strings.Join(block, "\n"),
			})
		}
	}

	out.Description = strings.Join(descLines, "\n")
	return out
}

// readUtil reads lines until 'pred' returns true.
//
// Returns the lines read as 'read' and whatever left as 'left'. When returns,
// 'left' is either empty or pred(&read[len(read)-1], left[0]) is true (where
// the pointer is actually nil if len(read) == 0).
//
// 'prev' is a line before the currently examined line or nil if the currently
// examined line is the first in 'in'.
func readUntil(in []string, pred func(prev *string, line string) (stop bool)) (read, left []string) {
	var prev *string
	idx := 0
	for idx < len(in) && !pred(prev, in[idx]) {
		prev = &in[idx]
		idx++
	}
	return in[:idx], in[idx:]
}

// parseFields parses a block of lines that define fields.
//
// It looks like this:
//
//    arg1: blah-blah,
//       maybe more-blah-blah.
//    arg2: shorter blah-blah.
//
//    arg3:
func parseFields(lines []string) []Field {
	var fields []Field

	for len(lines) > 0 {
		// Grab the name of the field from the first line.
		name, firstLine, ok := parseFieldLine(lines[0])
		if !ok {
			break
		}
		lines = lines[1:]

		// All other lines of the field description (if any) are intended.
		var block []string
		block, lines = readUntil(lines, func(_ *string, l string) bool {
			return l != "" && !hasLeadingSpace(l)
		})

		// Combine the first line with the rest of the block.
		all := trimEmptyLines(append([]string{firstLine}, deindent(block)...))

		// Join lines by space. We assume argument descriptions do not use newlines
		// in a syntax-significant way.
		fields = append(fields, Field{
			Name: name,
			Desc: strings.Join(all, " "),
		})
	}

	return fields
}

// //////////////////////////////////////////////////////////////////////////////
// Lower level utilities.

// normalizedLines takes a docstring literal and returns deindented cleaned
// up lines.
//
// E.g. this:
//
//   """Blah blah
//
//   More blah.<space><space>
//
//   """
//
// Results in ["blah blah", "", "More blah."].
func normalizedLines(doc string) []string {
	// Get rid of trailing whitespaces right away, they are insignificant.
	lines := strings.Split(doc, "\n")
	for idx, l := range lines {
		lines[idx] = strings.TrimRightFunc(l, unicode.IsSpace)
	}

	// Get rid of all leading and trailing empty lines, they are insignificant
	// too and just complicate life.
	lines = trimEmptyLines(lines)
	if len(lines) == 0 {
		return nil
	}

	// 'lines' here is something like:
	//
	// ["This function is blah-blah, see", "  also blah", "", "  More blah"].
	//
	// This is because docstrings do indentation only starting from the second
	// line. Just to make sure, we strip leading space from the first line. Then
	// we deindent the rest.
	lines[0] = strings.TrimLeftFunc(lines[0], unicode.IsSpace)
	deindent(lines[1:])
	return lines
}

// trimEmptyLines removes leading and trailing empty lines from the slice.
func trimEmptyLines(lines []string) []string {
	for len(lines) > 0 && lines[0] == "" {
		lines = lines[1:]
	}
	for len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	return lines
}

// deindent removes common indentation from the lines.
//
// Mutates the slice in-place and returns it.
//
// Whitespace only lines are replaced by completely empty lines. All whitespace
// characters are treated equally as 1 indentation level, so mixed spaces and
// tabs will result in a weird output (but using ether spaces or either tabs
// alone is fine).
func deindent(lines []string) []string {
	const inf = 9999999

	// First pass: get rid of whitespace-only lines, find the smallest indentation
	// level (in number of whitespace runes) among non-empty lines.
	numRunesToSkip := inf
	for idx, l := range lines {
		spaceRunes := countLeadingSpace(l)
		if spaceRunes == utf8.RuneCountInString(l) {
			lines[idx] = "" // clear completely whitespace lines
		} else if spaceRunes < numRunesToSkip {
			numRunesToSkip = spaceRunes
		}
	}

	// No lines at all, or only empty lines, or nothing to deindent.
	if numRunesToSkip == inf || numRunesToSkip == 0 {
		return lines
	}

	bytesToSkip := func(s string) int {
		idx := 0
		for pos := range s {
			if idx == numRunesToSkip {
				return pos
			}
			idx++
		}
		panic("unreachable")
	}

	// Cut the indentation by skipping 'numRunesToSkip' number of runes in each
	// non-empty line.
	for idx, l := range lines {
		if l != "" {
			lines[idx] = l[bytesToSkip(l):]
		}
	}
	return lines
}

// countLeadingSpace returns number of space runes at the prefix of a string.
func countLeadingSpace(s string) (runes int) {
	for _, r := range s {
		if !unicode.IsSpace(r) {
			break
		}
		runes++
	}
	return
}

// hasLeadingSpace returns true if 's' starts with a space.
func hasLeadingSpace(s string) bool {
	for _, r := range s {
		return unicode.IsSpace(r)
	}
	return false
}

// parseBlockTitle recognizes strings like "Args:" that indicate the beginning
// of a named block.
//
// We are pretty strict here to avoid misfiring on ':' that appear in sentences:
//   * The title should start with the upper case ("Args", not "args").
//   * No spaces allowed ("Returns", not "Return value").
func parseBlockTitle(l string) (title string, ok bool) {
	t := strings.TrimSuffix(l, ":")
	if len(t) == len(l) || len(t) == 0 {
		return // doesn't end in ':' or just ':' itself
	}
	// Check it has no space and the first rune is capital.
	for pos, r := range t {
		if unicode.IsSpace(r) || (pos == 0 && !unicode.IsUpper(r)) {
			return
		}
	}
	return t, true
}

// fieldRe matches "field: ...".
var fieldRe = regexp.MustCompile(`^(\S*)\s*:\s*(.*)$`)

// parseFieldLine recognized strings like "field<space>*:<space>*...".
//
// Returns the extracted field name and what is left of the line.
//
// No spaces are allowed in the field name part.
func parseFieldLine(l string) (field, rest string, ok bool) {
	if m := fieldRe.FindStringSubmatch(l); m != nil {
		return m[1], m[2], true
	}
	return
}
