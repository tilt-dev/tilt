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

import (
	"bytes"
)

// The parser implements the following grammar:
//
// stream               ::= STREAM-START implicit_document? explicit_document* STREAM-END
// implicit_document    ::= block_node DOCUMENT-END*
// explicit_document    ::= DIRECTIVE* DOCUMENT-START block_node? DOCUMENT-END*
// block_node_or_indentless_sequence    ::=
//                          ALIAS
//                          | properties (block_content | indentless_block_sequence)?
//                          | block_content
//                          | indentless_block_sequence
// block_node           ::= ALIAS
//                          | properties block_content?
//                          | block_content
// flow_node            ::= ALIAS
//                          | properties flow_content?
//                          | flow_content
// properties           ::= TAG ANCHOR? | ANCHOR TAG?
// block_content        ::= block_collection | flow_collection | SCALAR
// flow_content         ::= flow_collection | SCALAR
// block_collection     ::= block_sequence | block_mapping
// flow_collection      ::= flow_sequence | flow_mapping
// block_sequence       ::= BLOCK-SEQUENCE-START (BLOCK-ENTRY block_node?)* BLOCK-END
// indentless_sequence  ::= (BLOCK-ENTRY block_node?)+
// block_mapping        ::= BLOCK-MAPPING_START
//                          ((KEY block_node_or_indentless_sequence?)?
//                          (VALUE block_node_or_indentless_sequence?)?)*
//                          BLOCK-END
// flow_sequence        ::= FLOW-SEQUENCE-START
//                          (flow_sequence_entry FLOW-ENTRY)*
//                          flow_sequence_entry?
//                          FLOW-SEQUENCE-END
// flow_sequence_entry  ::= flow_node | KEY flow_node? (VALUE flow_node?)?
// flow_mapping         ::= FLOW-MAPPING-START
//                          (flow_mapping_entry FLOW-ENTRY)*
//                          flow_mapping_entry?
//                          FLOW-MAPPING-END
// flow_mapping_entry   ::= flow_node | KEY flow_node? (VALUE flow_node?)?

// Peek the next token in the token queue.
func (parser *Parser) peekToken() *Token {
	if parser.token_available || parser.fetchMoreTokens() {
		token := &parser.tokens[parser.tokens_head]
		parser.UnfoldComments(token)
		return token
	}
	return nil
}

// UnfoldComments walks through the comments queue and joins all
// comments behind the position of the provided token into the respective
// top-level comment slices in the parser.
func (parser *Parser) UnfoldComments(token *Token) {
	for parser.comments_head < len(parser.comments) && token.StartMark.Index >= parser.comments[parser.comments_head].token_mark.Index {
		comment := &parser.comments[parser.comments_head]
		if len(comment.head) > 0 {
			if token.Type == BLOCK_END_TOKEN {
				// No heads on ends, so keep comment.head for a follow up token.
				break
			}
			if len(parser.HeadComment) > 0 {
				parser.HeadComment = append(parser.HeadComment, '\n')
			}
			parser.HeadComment = append(parser.HeadComment, comment.head...)
		}
		if len(comment.foot) > 0 {
			if len(parser.FootComment) > 0 {
				parser.FootComment = append(parser.FootComment, '\n')
			}
			parser.FootComment = append(parser.FootComment, comment.foot...)
		}
		if len(comment.line) > 0 {
			if len(parser.LineComment) > 0 {
				parser.LineComment = append(parser.LineComment, '\n')
			}
			parser.LineComment = append(parser.LineComment, comment.line...)
		}
		*comment = Comment{}
		parser.comments_head++
	}
}

// Remove the next token from the queue (must be called after peek_token).
func (parser *Parser) skipToken() {
	parser.token_available = false
	parser.tokens_parsed++
	parser.stream_end_produced = parser.tokens[parser.tokens_head].Type == STREAM_END_TOKEN
	parser.tokens_head++
}

// Parse gets the next event.
func (parser *Parser) Parse(event *Event) bool {
	// Erase the event object.
	*event = Event{}

	// No events after the end of the stream or error.
	if parser.stream_end_produced || parser.ErrorType != NO_ERROR || parser.state == PARSE_END_STATE {
		return true
	}

	// Generate the next event.
	return parser.stateMachine(event)
}

// Set parser error.
func (parser *Parser) setParserError(problem string, problem_mark Mark) bool {
	parser.ErrorType = PARSER_ERROR
	parser.Problem = problem
	parser.ProblemMark = problem_mark
	return false
}

func (parser *Parser) setParserErrorContext(context string, context_mark Mark, problem string, problem_mark Mark) bool {
	parser.ErrorType = PARSER_ERROR
	parser.Context = context
	parser.ContextMark = context_mark
	parser.Problem = problem
	parser.ProblemMark = problem_mark
	return false
}

// State dispatcher.
func (parser *Parser) stateMachine(event *Event) bool {
	// trace("yaml_parser_state_machine", "state:", parser.state.String())

	switch parser.state {
	case PARSE_STREAM_START_STATE:
		return parser.parseStreamStart(event)

	case PARSE_IMPLICIT_DOCUMENT_START_STATE:
		return parser.parseDocumentStart(event, true)

	case PARSE_DOCUMENT_START_STATE:
		return parser.parseDocumentStart(event, false)

	case PARSE_DOCUMENT_CONTENT_STATE:
		return parser.parseDocumentContent(event)

	case PARSE_DOCUMENT_END_STATE:
		return parser.parseDocumentEnd(event)

	case PARSE_BLOCK_NODE_STATE:
		return parser.parseNode(event, true, false)

	case PARSE_BLOCK_SEQUENCE_FIRST_ENTRY_STATE:
		return parser.parseBlockSequenceEntry(event, true)

	case PARSE_BLOCK_SEQUENCE_ENTRY_STATE:
		return parser.parseBlockSequenceEntry(event, false)

	case PARSE_INDENTLESS_SEQUENCE_ENTRY_STATE:
		return parser.parseIndentlessSequenceEntry(event)

	case PARSE_BLOCK_MAPPING_FIRST_KEY_STATE:
		return parser.parseBlockMappingKey(event, true)

	case PARSE_BLOCK_MAPPING_KEY_STATE:
		return parser.parseBlockMappingKey(event, false)

	case PARSE_BLOCK_MAPPING_VALUE_STATE:
		return parser.parseBlockMappingValue(event)

	case PARSE_FLOW_SEQUENCE_FIRST_ENTRY_STATE:
		return parser.parseFlowSequenceEntry(event, true)

	case PARSE_FLOW_SEQUENCE_ENTRY_STATE:
		return parser.parseFlowSequenceEntry(event, false)

	case PARSE_FLOW_SEQUENCE_ENTRY_MAPPING_KEY_STATE:
		return parser.parseFlowSequenceEntryMappingKey(event)

	case PARSE_FLOW_SEQUENCE_ENTRY_MAPPING_VALUE_STATE:
		return parser.parseFlowSequenceEntryMappingValue(event)

	case PARSE_FLOW_SEQUENCE_ENTRY_MAPPING_END_STATE:
		return parser.parseFlowSequenceEntryMappingEnd(event)

	case PARSE_FLOW_MAPPING_FIRST_KEY_STATE:
		return parser.parseFlowMappingKey(event, true)

	case PARSE_FLOW_MAPPING_KEY_STATE:
		return parser.parseFlowMappingKey(event, false)

	case PARSE_FLOW_MAPPING_VALUE_STATE:
		return parser.parseFlowMappingValue(event, false)

	case PARSE_FLOW_MAPPING_EMPTY_VALUE_STATE:
		return parser.parseFlowMappingValue(event, true)

	default:
		panic("invalid parser state")
	}
}

// Parse the production:
// stream   ::= STREAM-START implicit_document? explicit_document* STREAM-END
//
//	************
func (parser *Parser) parseStreamStart(event *Event) bool {
	token := parser.peekToken()
	if token == nil {
		return false
	}
	if token.Type != STREAM_START_TOKEN {
		return parser.setParserError("did not find expected <stream-start>", token.StartMark)
	}
	parser.state = PARSE_IMPLICIT_DOCUMENT_START_STATE
	*event = Event{
		Type:      STREAM_START_EVENT,
		StartMark: token.StartMark,
		EndMark:   token.EndMark,
		encoding:  token.encoding,
	}
	parser.skipToken()
	return true
}

// Parse the productions:
// implicit_document    ::= block_node DOCUMENT-END*
//
//	*
//
// explicit_document    ::= DIRECTIVE* DOCUMENT-START block_node? DOCUMENT-END*
//
//	*************************
func (parser *Parser) parseDocumentStart(event *Event, implicit bool) bool {
	token := parser.peekToken()
	if token == nil {
		return false
	}

	// Parse extra document end indicators.
	if !implicit {
		for token.Type == DOCUMENT_END_TOKEN {
			parser.skipToken()
			token = parser.peekToken()
			if token == nil {
				return false
			}
		}
	}

	if implicit && token.Type != VERSION_DIRECTIVE_TOKEN &&
		token.Type != TAG_DIRECTIVE_TOKEN &&
		token.Type != DOCUMENT_START_TOKEN &&
		token.Type != STREAM_END_TOKEN {
		// Parse an implicit document.
		if !parser.processDirectives(nil, nil) {
			return false
		}
		parser.states = append(parser.states, PARSE_DOCUMENT_END_STATE)
		parser.state = PARSE_BLOCK_NODE_STATE

		var head_comment []byte
		if len(parser.HeadComment) > 0 {
			// [Go] Scan the header comment backwards, and if an empty line is found, break
			//      the header so the part before the last empty line goes into the
			//      document header, while the bottom of it goes into a follow up event.
			for i := len(parser.HeadComment) - 1; i > 0; i-- {
				if parser.HeadComment[i] == '\n' {
					if i == len(parser.HeadComment)-1 {
						head_comment = parser.HeadComment[:i]
						parser.HeadComment = parser.HeadComment[i+1:]
						break
					} else if parser.HeadComment[i-1] == '\n' {
						head_comment = parser.HeadComment[:i-1]
						parser.HeadComment = parser.HeadComment[i+1:]
						break
					}
				}
			}
		}

		*event = Event{
			Type:      DOCUMENT_START_EVENT,
			StartMark: token.StartMark,
			EndMark:   token.EndMark,
			Implicit:  true,

			HeadComment: head_comment,
		}

	} else if token.Type != STREAM_END_TOKEN {
		// Parse an explicit document.
		var version_directive *VersionDirective
		var tag_directives []TagDirective
		start_mark := token.StartMark
		if !parser.processDirectives(&version_directive, &tag_directives) {
			return false
		}
		token = parser.peekToken()
		if token == nil {
			return false
		}
		if token.Type != DOCUMENT_START_TOKEN {
			parser.setParserError(
				"did not find expected <document start>", token.StartMark)
			return false
		}
		parser.states = append(parser.states, PARSE_DOCUMENT_END_STATE)
		parser.state = PARSE_DOCUMENT_CONTENT_STATE
		end_mark := token.EndMark

		*event = Event{
			Type:              DOCUMENT_START_EVENT,
			StartMark:         start_mark,
			EndMark:           end_mark,
			version_directive: version_directive,
			tag_directives:    tag_directives,
			Implicit:          false,
		}
		parser.skipToken()

	} else {
		// Parse the stream end.
		parser.state = PARSE_END_STATE
		*event = Event{
			Type:      STREAM_END_EVENT,
			StartMark: token.StartMark,
			EndMark:   token.EndMark,
		}
		parser.skipToken()
	}

	return true
}

// Parse the productions:
// explicit_document    ::= DIRECTIVE* DOCUMENT-START block_node? DOCUMENT-END*
//
//	***********
func (parser *Parser) parseDocumentContent(event *Event) bool {
	token := parser.peekToken()
	if token == nil {
		return false
	}

	if token.Type == VERSION_DIRECTIVE_TOKEN ||
		token.Type == TAG_DIRECTIVE_TOKEN ||
		token.Type == DOCUMENT_START_TOKEN ||
		token.Type == DOCUMENT_END_TOKEN ||
		token.Type == STREAM_END_TOKEN {
		parser.state = parser.states[len(parser.states)-1]
		parser.states = parser.states[:len(parser.states)-1]
		return parser.processEmptyScalar(event,
			token.StartMark)
	}
	return parser.parseNode(event, true, false)
}

// Parse the productions:
// implicit_document    ::= block_node DOCUMENT-END*
//
//	*************
//
// explicit_document    ::= DIRECTIVE* DOCUMENT-START block_node? DOCUMENT-END*
func (parser *Parser) parseDocumentEnd(event *Event) bool {
	token := parser.peekToken()
	if token == nil {
		return false
	}

	start_mark := token.StartMark
	end_mark := token.StartMark

	implicit := true
	if token.Type == DOCUMENT_END_TOKEN {
		end_mark = token.EndMark
		parser.skipToken()
		implicit = false
	}

	parser.tag_directives = parser.tag_directives[:0]

	parser.state = PARSE_DOCUMENT_START_STATE
	*event = Event{
		Type:      DOCUMENT_END_EVENT,
		StartMark: start_mark,
		EndMark:   end_mark,
		Implicit:  implicit,
	}
	parser.setEventComments(event)
	if len(event.HeadComment) > 0 && len(event.FootComment) == 0 {
		event.FootComment = event.HeadComment
		event.HeadComment = nil
	}
	return true
}

func (parser *Parser) setEventComments(event *Event) {
	event.HeadComment = parser.HeadComment
	event.LineComment = parser.LineComment
	event.FootComment = parser.FootComment
	parser.HeadComment = nil
	parser.LineComment = nil
	parser.FootComment = nil
	parser.tail_comment = nil
	parser.stem_comment = nil
}

// Parse the productions:
// block_node_or_indentless_sequence    ::=
//
//	ALIAS
//	*****
//	| properties (block_content | indentless_block_sequence)?
//	  **********  *
//	| block_content | indentless_block_sequence
//	  *
//
// block_node           ::= ALIAS
//
//	*****
//	| properties block_content?
//	  ********** *
//	| block_content
//	  *
//
// flow_node            ::= ALIAS
//
//	*****
//	| properties flow_content?
//	  ********** *
//	| flow_content
//	  *
//
// properties           ::= TAG ANCHOR? | ANCHOR TAG?
//
//	*************************
//
// block_content        ::= block_collection | flow_collection | SCALAR
//
//	******
//
// flow_content         ::= flow_collection | SCALAR
//
//	******
func (parser *Parser) parseNode(event *Event, block, indentless_sequence bool) bool {
	// defer trace("yaml_parser_parse_node", "block:", block, "indentless_sequence:", indentless_sequence)()

	token := parser.peekToken()
	if token == nil {
		return false
	}

	if token.Type == ALIAS_TOKEN {
		parser.state = parser.states[len(parser.states)-1]
		parser.states = parser.states[:len(parser.states)-1]
		*event = Event{
			Type:      ALIAS_EVENT,
			StartMark: token.StartMark,
			EndMark:   token.EndMark,
			Anchor:    token.Value,
		}
		parser.setEventComments(event)
		parser.skipToken()
		return true
	}

	start_mark := token.StartMark
	end_mark := token.StartMark

	var tag_token bool
	var tag_handle, tag_suffix, anchor []byte
	var tag_mark Mark
	switch token.Type {
	case ANCHOR_TOKEN:
		anchor = token.Value
		start_mark = token.StartMark
		end_mark = token.EndMark
		parser.skipToken()
		token = parser.peekToken()
		if token == nil {
			return false
		}
		if token.Type == TAG_TOKEN {
			tag_token = true
			tag_handle = token.Value
			tag_suffix = token.suffix
			tag_mark = token.StartMark
			end_mark = token.EndMark
			parser.skipToken()
			token = parser.peekToken()
			if token == nil {
				return false
			}
		}
	case TAG_TOKEN:
		tag_token = true
		tag_handle = token.Value
		tag_suffix = token.suffix
		start_mark = token.StartMark
		tag_mark = token.StartMark
		end_mark = token.EndMark
		parser.skipToken()
		token = parser.peekToken()
		if token == nil {
			return false
		}
		if token.Type == ANCHOR_TOKEN {
			anchor = token.Value
			end_mark = token.EndMark
			parser.skipToken()
			token = parser.peekToken()
			if token == nil {
				return false
			}
		}
	}

	var tag []byte
	if tag_token {
		if len(tag_handle) == 0 {
			tag = tag_suffix
		} else {
			for i := range parser.tag_directives {
				if bytes.Equal(parser.tag_directives[i].handle, tag_handle) {
					tag = append([]byte(nil), parser.tag_directives[i].prefix...)
					tag = append(tag, tag_suffix...)
					break
				}
			}
			if len(tag) == 0 {
				parser.setParserErrorContext(
					"while parsing a node", start_mark,
					"found undefined tag handle", tag_mark)
				return false
			}
		}
	}

	implicit := len(tag) == 0
	if indentless_sequence && token.Type == BLOCK_ENTRY_TOKEN {
		end_mark = token.EndMark
		parser.state = PARSE_INDENTLESS_SEQUENCE_ENTRY_STATE
		*event = Event{
			Type:      SEQUENCE_START_EVENT,
			StartMark: start_mark,
			EndMark:   end_mark,
			Anchor:    anchor,
			Tag:       tag,
			Implicit:  implicit,
			Style:     Style(BLOCK_SEQUENCE_STYLE),
		}
		return true
	}
	if token.Type == SCALAR_TOKEN {
		var plain_implicit, quoted_implicit bool
		end_mark = token.EndMark
		if (len(tag) == 0 && token.Style == PLAIN_SCALAR_STYLE) || (len(tag) == 1 && tag[0] == '!') {
			plain_implicit = true
		} else if len(tag) == 0 {
			quoted_implicit = true
		}
		parser.state = parser.states[len(parser.states)-1]
		parser.states = parser.states[:len(parser.states)-1]

		*event = Event{
			Type:            SCALAR_EVENT,
			StartMark:       start_mark,
			EndMark:         end_mark,
			Anchor:          anchor,
			Tag:             tag,
			Value:           token.Value,
			Implicit:        plain_implicit,
			quoted_implicit: quoted_implicit,
			Style:           Style(token.Style),
		}
		parser.setEventComments(event)
		parser.skipToken()
		return true
	}
	if token.Type == FLOW_SEQUENCE_START_TOKEN {
		// [Go] Some of the events below can be merged as they differ only on style.
		end_mark = token.EndMark
		parser.state = PARSE_FLOW_SEQUENCE_FIRST_ENTRY_STATE
		*event = Event{
			Type:      SEQUENCE_START_EVENT,
			StartMark: start_mark,
			EndMark:   end_mark,
			Anchor:    anchor,
			Tag:       tag,
			Implicit:  implicit,
			Style:     Style(FLOW_SEQUENCE_STYLE),
		}
		parser.setEventComments(event)
		return true
	}
	if token.Type == FLOW_MAPPING_START_TOKEN {
		end_mark = token.EndMark
		parser.state = PARSE_FLOW_MAPPING_FIRST_KEY_STATE
		*event = Event{
			Type:      MAPPING_START_EVENT,
			StartMark: start_mark,
			EndMark:   end_mark,
			Anchor:    anchor,
			Tag:       tag,
			Implicit:  implicit,
			Style:     Style(FLOW_MAPPING_STYLE),
		}
		parser.setEventComments(event)
		return true
	}
	if block && token.Type == BLOCK_SEQUENCE_START_TOKEN {
		end_mark = token.EndMark
		parser.state = PARSE_BLOCK_SEQUENCE_FIRST_ENTRY_STATE
		*event = Event{
			Type:      SEQUENCE_START_EVENT,
			StartMark: start_mark,
			EndMark:   end_mark,
			Anchor:    anchor,
			Tag:       tag,
			Implicit:  implicit,
			Style:     Style(BLOCK_SEQUENCE_STYLE),
		}
		if parser.stem_comment != nil {
			event.HeadComment = parser.stem_comment
			parser.stem_comment = nil
		}
		return true
	}
	if block && token.Type == BLOCK_MAPPING_START_TOKEN {
		end_mark = token.EndMark
		parser.state = PARSE_BLOCK_MAPPING_FIRST_KEY_STATE
		*event = Event{
			Type:      MAPPING_START_EVENT,
			StartMark: start_mark,
			EndMark:   end_mark,
			Anchor:    anchor,
			Tag:       tag,
			Implicit:  implicit,
			Style:     Style(BLOCK_MAPPING_STYLE),
		}
		if parser.stem_comment != nil {
			event.HeadComment = parser.stem_comment
			parser.stem_comment = nil
		}
		return true
	}
	if len(anchor) > 0 || len(tag) > 0 {
		parser.state = parser.states[len(parser.states)-1]
		parser.states = parser.states[:len(parser.states)-1]

		*event = Event{
			Type:            SCALAR_EVENT,
			StartMark:       start_mark,
			EndMark:         end_mark,
			Anchor:          anchor,
			Tag:             tag,
			Implicit:        implicit,
			quoted_implicit: false,
			Style:           Style(PLAIN_SCALAR_STYLE),
		}
		return true
	}

	context := "while parsing a flow node"
	if block {
		context = "while parsing a block node"
	}
	parser.setParserErrorContext(context, start_mark,
		"did not find expected node content", token.StartMark)
	return false
}

// Parse the productions:
// block_sequence ::= BLOCK-SEQUENCE-START (BLOCK-ENTRY block_node?)* BLOCK-END
//
//	********************  *********** *             *********
func (parser *Parser) parseBlockSequenceEntry(event *Event, first bool) bool {
	if first {
		token := parser.peekToken()
		if token == nil {
			return false
		}
		parser.marks = append(parser.marks, token.StartMark)
		parser.skipToken()
	}

	token := parser.peekToken()
	if token == nil {
		return false
	}

	if token.Type == BLOCK_ENTRY_TOKEN {
		mark := token.EndMark
		prior_head_len := len(parser.HeadComment)
		parser.skipToken()
		parser.splitStemComment(prior_head_len)
		token = parser.peekToken()
		if token == nil {
			return false
		}
		if token.Type != BLOCK_ENTRY_TOKEN && token.Type != BLOCK_END_TOKEN {
			parser.states = append(parser.states, PARSE_BLOCK_SEQUENCE_ENTRY_STATE)
			return parser.parseNode(event, true, false)
		} else {
			parser.state = PARSE_BLOCK_SEQUENCE_ENTRY_STATE
			return parser.processEmptyScalar(event, mark)
		}
	}
	if token.Type == BLOCK_END_TOKEN {
		parser.state = parser.states[len(parser.states)-1]
		parser.states = parser.states[:len(parser.states)-1]
		parser.marks = parser.marks[:len(parser.marks)-1]

		*event = Event{
			Type:      SEQUENCE_END_EVENT,
			StartMark: token.StartMark,
			EndMark:   token.EndMark,
		}

		parser.skipToken()
		return true
	}

	context_mark := parser.marks[len(parser.marks)-1]
	parser.marks = parser.marks[:len(parser.marks)-1]
	return parser.setParserErrorContext(
		"while parsing a block collection", context_mark,
		"did not find expected '-' indicator", token.StartMark)
}

// Parse the productions:
// indentless_sequence  ::= (BLOCK-ENTRY block_node?)+
//
//	*********** *
func (parser *Parser) parseIndentlessSequenceEntry(event *Event) bool {
	token := parser.peekToken()
	if token == nil {
		return false
	}

	if token.Type == BLOCK_ENTRY_TOKEN {
		mark := token.EndMark
		prior_head_len := len(parser.HeadComment)
		parser.skipToken()
		parser.splitStemComment(prior_head_len)
		token = parser.peekToken()
		if token == nil {
			return false
		}
		if token.Type != BLOCK_ENTRY_TOKEN &&
			token.Type != KEY_TOKEN &&
			token.Type != VALUE_TOKEN &&
			token.Type != BLOCK_END_TOKEN {
			parser.states = append(parser.states, PARSE_INDENTLESS_SEQUENCE_ENTRY_STATE)
			return parser.parseNode(event, true, false)
		}
		parser.state = PARSE_INDENTLESS_SEQUENCE_ENTRY_STATE
		return parser.processEmptyScalar(event, mark)
	}
	parser.state = parser.states[len(parser.states)-1]
	parser.states = parser.states[:len(parser.states)-1]

	*event = Event{
		Type:      SEQUENCE_END_EVENT,
		StartMark: token.StartMark,
		EndMark:   token.StartMark, // [Go] Shouldn't this be token.end_mark?
	}
	return true
}

// Split stem comment from head comment.
//
// When a sequence or map is found under a sequence entry, the former head comment
// is assigned to the underlying sequence or map as a whole, not the individual
// sequence or map entry as would be expected otherwise. To handle this case the
// previous head comment is moved aside as the stem comment.
func (parser *Parser) splitStemComment(stem_len int) {
	if stem_len == 0 {
		return
	}

	token := parser.peekToken()
	if token == nil || token.Type != BLOCK_SEQUENCE_START_TOKEN && token.Type != BLOCK_MAPPING_START_TOKEN {
		return
	}

	parser.stem_comment = parser.HeadComment[:stem_len]
	if len(parser.HeadComment) == stem_len {
		parser.HeadComment = nil
	} else {
		// Copy suffix to prevent very strange bugs if someone ever appends
		// further bytes to the prefix in the stem_comment slice above.
		parser.HeadComment = append([]byte(nil), parser.HeadComment[stem_len+1:]...)
	}
}

// Parse the productions:
// block_mapping        ::= BLOCK-MAPPING_START
//
//	*******************
//	((KEY block_node_or_indentless_sequence?)?
//	  *** *
//	(VALUE block_node_or_indentless_sequence?)?)*
//
//	BLOCK-END
//	*********
func (parser *Parser) parseBlockMappingKey(event *Event, first bool) bool {
	if first {
		token := parser.peekToken()
		if token == nil {
			return false
		}
		parser.marks = append(parser.marks, token.StartMark)
		parser.skipToken()
	}

	token := parser.peekToken()
	if token == nil {
		return false
	}

	// [Go] A tail comment was left from the prior mapping value processed. Emit an event
	//      as it needs to be processed with that value and not the following key.
	if len(parser.tail_comment) > 0 {
		*event = Event{
			Type:        TAIL_COMMENT_EVENT,
			StartMark:   token.StartMark,
			EndMark:     token.EndMark,
			FootComment: parser.tail_comment,
		}
		parser.tail_comment = nil
		return true
	}

	switch token.Type {
	case KEY_TOKEN:
		mark := token.EndMark
		parser.skipToken()
		token = parser.peekToken()
		if token == nil {
			return false
		}
		if token.Type != KEY_TOKEN &&
			token.Type != VALUE_TOKEN &&
			token.Type != BLOCK_END_TOKEN {
			parser.states = append(parser.states, PARSE_BLOCK_MAPPING_VALUE_STATE)
			return parser.parseNode(event, true, true)
		} else {
			parser.state = PARSE_BLOCK_MAPPING_VALUE_STATE
			return parser.processEmptyScalar(event, mark)
		}
	case BLOCK_END_TOKEN:
		parser.state = parser.states[len(parser.states)-1]
		parser.states = parser.states[:len(parser.states)-1]
		parser.marks = parser.marks[:len(parser.marks)-1]
		*event = Event{
			Type:      MAPPING_END_EVENT,
			StartMark: token.StartMark,
			EndMark:   token.EndMark,
		}
		parser.setEventComments(event)
		parser.skipToken()
		return true
	}

	context_mark := parser.marks[len(parser.marks)-1]
	parser.marks = parser.marks[:len(parser.marks)-1]
	return parser.setParserErrorContext(
		"while parsing a block mapping", context_mark,
		"did not find expected key", token.StartMark)
}

// Parse the productions:
// block_mapping        ::= BLOCK-MAPPING_START
//
//	((KEY block_node_or_indentless_sequence?)?
//
//	(VALUE block_node_or_indentless_sequence?)?)*
//	 ***** *
//	BLOCK-END
func (parser *Parser) parseBlockMappingValue(event *Event) bool {
	token := parser.peekToken()
	if token == nil {
		return false
	}
	if token.Type == VALUE_TOKEN {
		mark := token.EndMark
		parser.skipToken()
		token = parser.peekToken()
		if token == nil {
			return false
		}
		if token.Type != KEY_TOKEN &&
			token.Type != VALUE_TOKEN &&
			token.Type != BLOCK_END_TOKEN {
			parser.states = append(parser.states, PARSE_BLOCK_MAPPING_KEY_STATE)
			return parser.parseNode(event, true, true)
		}
		parser.state = PARSE_BLOCK_MAPPING_KEY_STATE
		return parser.processEmptyScalar(event, mark)
	}
	parser.state = PARSE_BLOCK_MAPPING_KEY_STATE
	return parser.processEmptyScalar(event, token.StartMark)
}

// Parse the productions:
// flow_sequence        ::= FLOW-SEQUENCE-START
//
//	*******************
//	(flow_sequence_entry FLOW-ENTRY)*
//	 *                   **********
//	flow_sequence_entry?
//	*
//	FLOW-SEQUENCE-END
//	*****************
//
// flow_sequence_entry  ::= flow_node | KEY flow_node? (VALUE flow_node?)?
//
//	*
func (parser *Parser) parseFlowSequenceEntry(event *Event, first bool) bool {
	if first {
		token := parser.peekToken()
		if token == nil {
			return false
		}
		parser.marks = append(parser.marks, token.StartMark)
		parser.skipToken()
	}
	token := parser.peekToken()
	if token == nil {
		return false
	}
	if token.Type != FLOW_SEQUENCE_END_TOKEN {
		if !first {
			if token.Type == FLOW_ENTRY_TOKEN {
				parser.skipToken()
				token = parser.peekToken()
				if token == nil {
					return false
				}
			} else {
				context_mark := parser.marks[len(parser.marks)-1]
				parser.marks = parser.marks[:len(parser.marks)-1]
				return parser.setParserErrorContext(
					"while parsing a flow sequence", context_mark,
					"did not find expected ',' or ']'", token.StartMark)
			}
		}

		if token.Type == KEY_TOKEN {
			parser.state = PARSE_FLOW_SEQUENCE_ENTRY_MAPPING_KEY_STATE
			*event = Event{
				Type:      MAPPING_START_EVENT,
				StartMark: token.StartMark,
				EndMark:   token.EndMark,
				Implicit:  true,
				Style:     Style(FLOW_MAPPING_STYLE),
			}
			parser.skipToken()
			return true
		} else if token.Type != FLOW_SEQUENCE_END_TOKEN {
			parser.states = append(parser.states, PARSE_FLOW_SEQUENCE_ENTRY_STATE)
			return parser.parseNode(event, false, false)
		}
	}

	parser.state = parser.states[len(parser.states)-1]
	parser.states = parser.states[:len(parser.states)-1]
	parser.marks = parser.marks[:len(parser.marks)-1]

	*event = Event{
		Type:      SEQUENCE_END_EVENT,
		StartMark: token.StartMark,
		EndMark:   token.EndMark,
	}
	parser.setEventComments(event)

	parser.skipToken()
	return true
}

// Parse the productions:
// flow_sequence_entry  ::= flow_node | KEY flow_node? (VALUE flow_node?)?
//
//	*** *
func (parser *Parser) parseFlowSequenceEntryMappingKey(event *Event) bool {
	token := parser.peekToken()
	if token == nil {
		return false
	}
	if token.Type != VALUE_TOKEN &&
		token.Type != FLOW_ENTRY_TOKEN &&
		token.Type != FLOW_SEQUENCE_END_TOKEN {
		parser.states = append(parser.states, PARSE_FLOW_SEQUENCE_ENTRY_MAPPING_VALUE_STATE)
		return parser.parseNode(event, false, false)
	}
	mark := token.EndMark
	parser.skipToken()
	parser.state = PARSE_FLOW_SEQUENCE_ENTRY_MAPPING_VALUE_STATE
	return parser.processEmptyScalar(event, mark)
}

// Parse the productions:
// flow_sequence_entry  ::= flow_node | KEY flow_node? (VALUE flow_node?)?
//
//	***** *
func (parser *Parser) parseFlowSequenceEntryMappingValue(event *Event) bool {
	token := parser.peekToken()
	if token == nil {
		return false
	}
	if token.Type == VALUE_TOKEN {
		parser.skipToken()
		token := parser.peekToken()
		if token == nil {
			return false
		}
		if token.Type != FLOW_ENTRY_TOKEN && token.Type != FLOW_SEQUENCE_END_TOKEN {
			parser.states = append(parser.states, PARSE_FLOW_SEQUENCE_ENTRY_MAPPING_END_STATE)
			return parser.parseNode(event, false, false)
		}
	}
	parser.state = PARSE_FLOW_SEQUENCE_ENTRY_MAPPING_END_STATE
	return parser.processEmptyScalar(event, token.StartMark)
}

// Parse the productions:
// flow_sequence_entry  ::= flow_node | KEY flow_node? (VALUE flow_node?)?
//
//	*
func (parser *Parser) parseFlowSequenceEntryMappingEnd(event *Event) bool {
	token := parser.peekToken()
	if token == nil {
		return false
	}
	parser.state = PARSE_FLOW_SEQUENCE_ENTRY_STATE
	*event = Event{
		Type:      MAPPING_END_EVENT,
		StartMark: token.StartMark,
		EndMark:   token.StartMark, // [Go] Shouldn't this be end_mark?
	}
	return true
}

// Parse the productions:
// flow_mapping         ::= FLOW-MAPPING-START
//
//	******************
//	(flow_mapping_entry FLOW-ENTRY)*
//	 *                  **********
//	flow_mapping_entry?
//	******************
//	FLOW-MAPPING-END
//	****************
//
// flow_mapping_entry   ::= flow_node | KEY flow_node? (VALUE flow_node?)?
//   - *** *
func (parser *Parser) parseFlowMappingKey(event *Event, first bool) bool {
	if first {
		token := parser.peekToken()
		parser.marks = append(parser.marks, token.StartMark)
		parser.skipToken()
	}

	token := parser.peekToken()
	if token == nil {
		return false
	}

	if token.Type != FLOW_MAPPING_END_TOKEN {
		if !first {
			if token.Type == FLOW_ENTRY_TOKEN {
				parser.skipToken()
				token = parser.peekToken()
				if token == nil {
					return false
				}
			} else {
				context_mark := parser.marks[len(parser.marks)-1]
				parser.marks = parser.marks[:len(parser.marks)-1]
				return parser.setParserErrorContext(
					"while parsing a flow mapping", context_mark,
					"did not find expected ',' or '}'", token.StartMark)
			}
		}

		if token.Type == KEY_TOKEN {
			parser.skipToken()
			token = parser.peekToken()
			if token == nil {
				return false
			}
			if token.Type != VALUE_TOKEN &&
				token.Type != FLOW_ENTRY_TOKEN &&
				token.Type != FLOW_MAPPING_END_TOKEN {
				parser.states = append(parser.states, PARSE_FLOW_MAPPING_VALUE_STATE)
				return parser.parseNode(event, false, false)
			} else {
				parser.state = PARSE_FLOW_MAPPING_VALUE_STATE
				return parser.processEmptyScalar(event, token.StartMark)
			}
		} else if token.Type != FLOW_MAPPING_END_TOKEN {
			parser.states = append(parser.states, PARSE_FLOW_MAPPING_EMPTY_VALUE_STATE)
			return parser.parseNode(event, false, false)
		}
	}

	parser.state = parser.states[len(parser.states)-1]
	parser.states = parser.states[:len(parser.states)-1]
	parser.marks = parser.marks[:len(parser.marks)-1]
	*event = Event{
		Type:      MAPPING_END_EVENT,
		StartMark: token.StartMark,
		EndMark:   token.EndMark,
	}
	parser.setEventComments(event)
	parser.skipToken()
	return true
}

// Parse the productions:
// flow_mapping_entry   ::= flow_node | KEY flow_node? (VALUE flow_node?)?
//   - ***** *
func (parser *Parser) parseFlowMappingValue(event *Event, empty bool) bool {
	token := parser.peekToken()
	if token == nil {
		return false
	}
	if empty {
		parser.state = PARSE_FLOW_MAPPING_KEY_STATE
		return parser.processEmptyScalar(event, token.StartMark)
	}
	if token.Type == VALUE_TOKEN {
		parser.skipToken()
		token = parser.peekToken()
		if token == nil {
			return false
		}
		if token.Type != FLOW_ENTRY_TOKEN && token.Type != FLOW_MAPPING_END_TOKEN {
			parser.states = append(parser.states, PARSE_FLOW_MAPPING_KEY_STATE)
			return parser.parseNode(event, false, false)
		}
	}
	parser.state = PARSE_FLOW_MAPPING_KEY_STATE
	return parser.processEmptyScalar(event, token.StartMark)
}

// Generate an empty scalar event.
func (parser *Parser) processEmptyScalar(event *Event, mark Mark) bool {
	*event = Event{
		Type:      SCALAR_EVENT,
		StartMark: mark,
		EndMark:   mark,
		Value:     nil, // Empty
		Implicit:  true,
		Style:     Style(PLAIN_SCALAR_STYLE),
	}
	return true
}

var default_tag_directives = []TagDirective{
	{[]byte("!"), []byte("!")},
	{[]byte("!!"), []byte("tag:yaml.org,2002:")},
}

// Parse directives.
func (parser *Parser) processDirectives(version_directive_ref **VersionDirective, tag_directives_ref *[]TagDirective) bool {
	var version_directive *VersionDirective
	var tag_directives []TagDirective

	token := parser.peekToken()
	if token == nil {
		return false
	}

	for token.Type == VERSION_DIRECTIVE_TOKEN || token.Type == TAG_DIRECTIVE_TOKEN {
		switch token.Type {
		case VERSION_DIRECTIVE_TOKEN:
			if version_directive != nil {
				parser.setParserError(
					"found duplicate %YAML directive", token.StartMark)
				return false
			}
			if token.major != 1 || token.minor != 1 {
				parser.setParserError(
					"found incompatible YAML document", token.StartMark)
				return false
			}
			version_directive = &VersionDirective{
				major: token.major,
				minor: token.minor,
			}
		case TAG_DIRECTIVE_TOKEN:
			value := TagDirective{
				handle: token.Value,
				prefix: token.prefix,
			}
			if !parser.appendTagDirective(value, false, token.StartMark) {
				return false
			}
			tag_directives = append(tag_directives, value)
		}

		parser.skipToken()
		token = parser.peekToken()
		if token == nil {
			return false
		}
	}

	for i := range default_tag_directives {
		if !parser.appendTagDirective(default_tag_directives[i], true, token.StartMark) {
			return false
		}
	}

	if version_directive_ref != nil {
		*version_directive_ref = version_directive
	}
	if tag_directives_ref != nil {
		*tag_directives_ref = tag_directives
	}
	return true
}

// Append a tag directive to the directives stack.
func (parser *Parser) appendTagDirective(value TagDirective, allow_duplicates bool, mark Mark) bool {
	for i := range parser.tag_directives {
		if bytes.Equal(value.handle, parser.tag_directives[i].handle) {
			if allow_duplicates {
				return true
			}
			return parser.setParserError("found duplicate %TAG directive", mark)
		}
	}

	// [Go] I suspect the copy is unnecessary. This was likely done
	// because there was no way to track ownership of the data.
	value_copy := TagDirective{
		handle: make([]byte, len(value.handle)),
		prefix: make([]byte, len(value.prefix)),
	}
	copy(value_copy.handle, value.handle)
	copy(value_copy.prefix, value.prefix)
	parser.tag_directives = append(parser.tag_directives, value_copy)
	return true
}
