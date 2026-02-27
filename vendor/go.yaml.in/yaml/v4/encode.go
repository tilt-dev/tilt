//
// Copyright (c) 2011-2019 Canonical Ltd
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package yaml

import (
	"encoding"
	"fmt"
	"io"
	"reflect"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"go.yaml.in/yaml/v4/internal/libyaml"
)

type encoder struct {
	emitter  libyaml.Emitter
	event    libyaml.Event
	out      []byte
	flow     bool
	indent   int
	doneInit bool
}

func newEncoder() *encoder {
	e := &encoder{
		emitter: libyaml.NewEmitter(),
	}
	e.emitter.SetOutputString(&e.out)
	e.emitter.SetUnicode(true)
	return e
}

func newEncoderWithWriter(w io.Writer) *encoder {
	e := &encoder{
		emitter: libyaml.NewEmitter(),
	}
	e.emitter.SetOutputWriter(w)
	e.emitter.SetUnicode(true)
	return e
}

func (e *encoder) init() {
	if e.doneInit {
		return
	}
	if e.indent == 0 {
		e.indent = 4
	}
	e.emitter.BestIndent = e.indent
	e.event = libyaml.NewStreamStartEvent(libyaml.UTF8_ENCODING)
	e.emit()
	e.doneInit = true
}

func (e *encoder) finish() {
	e.emitter.OpenEnded = false
	e.event = libyaml.NewStreamEndEvent()
	e.emit()
}

func (e *encoder) destroy() {
	e.emitter.Delete()
}

func (e *encoder) emit() {
	// This will internally delete the e.event value.
	e.must(e.emitter.Emit(&e.event))
}

func (e *encoder) must(err error) {
	if err != nil {
		msg := err.Error()
		if msg == "" {
			msg = "unknown problem generating YAML content"
		}
		failf("%s", msg)
	}
}

func (e *encoder) marshalDoc(tag string, in reflect.Value) {
	e.init()
	var node *Node
	if in.IsValid() {
		node, _ = in.Interface().(*Node)
	}
	if node != nil && node.Kind == DocumentNode {
		e.nodev(in)
	} else {
		e.event = libyaml.NewDocumentStartEvent(nil, nil, true)
		e.emit()
		e.marshal(tag, in)
		e.event = libyaml.NewDocumentEndEvent(true)
		e.emit()
	}
}

func (e *encoder) marshal(tag string, in reflect.Value) {
	tag = shortTag(tag)
	if !in.IsValid() || in.Kind() == reflect.Pointer && in.IsNil() {
		e.nilv()
		return
	}
	iface := in.Interface()
	switch value := iface.(type) {
	case *Node:
		e.nodev(in)
		return
	case Node:
		if !in.CanAddr() {
			n := reflect.New(in.Type()).Elem()
			n.Set(in)
			in = n
		}
		e.nodev(in.Addr())
		return
	case time.Time:
		e.timev(tag, in)
		return
	case *time.Time:
		e.timev(tag, in.Elem())
		return
	case time.Duration:
		e.stringv(tag, reflect.ValueOf(value.String()))
		return
	case Marshaler:
		v, err := value.MarshalYAML()
		if err != nil {
			fail(err)
		}
		if v == nil {
			e.nilv()
			return
		}
		e.marshal(tag, reflect.ValueOf(v))
		return
	case encoding.TextMarshaler:
		text, err := value.MarshalText()
		if err != nil {
			fail(err)
		}
		in = reflect.ValueOf(string(text))
	case nil:
		e.nilv()
		return
	}
	switch in.Kind() {
	case reflect.Interface:
		e.marshal(tag, in.Elem())
	case reflect.Map:
		e.mapv(tag, in)
	case reflect.Pointer:
		e.marshal(tag, in.Elem())
	case reflect.Struct:
		e.structv(tag, in)
	case reflect.Slice, reflect.Array:
		e.slicev(tag, in)
	case reflect.String:
		e.stringv(tag, in)
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		e.intv(tag, in)
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		e.uintv(tag, in)
	case reflect.Float32, reflect.Float64:
		e.floatv(tag, in)
	case reflect.Bool:
		e.boolv(tag, in)
	default:
		panic("cannot marshal type: " + in.Type().String())
	}
}

func (e *encoder) mapv(tag string, in reflect.Value) {
	e.mappingv(tag, func() {
		keys := keyList(in.MapKeys())
		sort.Sort(keys)
		for _, k := range keys {
			e.marshal("", k)
			e.marshal("", in.MapIndex(k))
		}
	})
}

func (e *encoder) fieldByIndex(v reflect.Value, index []int) (field reflect.Value) {
	for _, num := range index {
		for {
			if v.Kind() == reflect.Pointer {
				if v.IsNil() {
					return reflect.Value{}
				}
				v = v.Elem()
				continue
			}
			break
		}
		v = v.Field(num)
	}
	return v
}

func (e *encoder) structv(tag string, in reflect.Value) {
	sinfo, err := getStructInfo(in.Type())
	if err != nil {
		panic(err)
	}
	e.mappingv(tag, func() {
		for _, info := range sinfo.FieldsList {
			var value reflect.Value
			if info.Inline == nil {
				value = in.Field(info.Num)
			} else {
				value = e.fieldByIndex(in, info.Inline)
				if !value.IsValid() {
					continue
				}
			}
			if info.OmitEmpty && isZero(value) {
				continue
			}
			e.marshal("", reflect.ValueOf(info.Key))
			e.flow = info.Flow
			e.marshal("", value)
		}
		if sinfo.InlineMap >= 0 {
			m := in.Field(sinfo.InlineMap)
			if m.Len() > 0 {
				e.flow = false
				keys := keyList(m.MapKeys())
				sort.Sort(keys)
				for _, k := range keys {
					if _, found := sinfo.FieldsMap[k.String()]; found {
						panic(fmt.Sprintf("cannot have key %q in inlined map: conflicts with struct field", k.String()))
					}
					e.marshal("", k)
					e.flow = false
					e.marshal("", m.MapIndex(k))
				}
			}
		}
	})
}

func (e *encoder) mappingv(tag string, f func()) {
	implicit := tag == ""
	style := libyaml.BLOCK_MAPPING_STYLE
	if e.flow {
		e.flow = false
		style = libyaml.FLOW_MAPPING_STYLE
	}
	e.event = libyaml.NewMappingStartEvent(nil, []byte(tag), implicit, style)
	e.emit()
	f()
	e.event = libyaml.NewMappingEndEvent()
	e.emit()
}

func (e *encoder) slicev(tag string, in reflect.Value) {
	implicit := tag == ""
	style := libyaml.BLOCK_SEQUENCE_STYLE
	if e.flow {
		e.flow = false
		style = libyaml.FLOW_SEQUENCE_STYLE
	}
	e.event = libyaml.NewSequenceStartEvent(nil, []byte(tag), implicit, style)
	e.emit()
	n := in.Len()
	for i := 0; i < n; i++ {
		e.marshal("", in.Index(i))
	}
	e.event = libyaml.NewSequenceEndEvent()
	e.emit()
}

// isBase60 returns whether s is in base 60 notation as defined in YAML 1.1.
//
// The base 60 float notation in YAML 1.1 is a terrible idea and is unsupported
// in YAML 1.2 and by this package, but these should be marshaled quoted for
// the time being for compatibility with other parsers.
func isBase60Float(s string) (result bool) {
	// Fast path.
	if s == "" {
		return false
	}
	c := s[0]
	if !(c == '+' || c == '-' || c >= '0' && c <= '9') || strings.IndexByte(s, ':') < 0 {
		return false
	}
	// Do the full match.
	return base60float.MatchString(s)
}

// From http://yaml.org/type/float.html, except the regular expression there
// is bogus. In practice parsers do not enforce the "\.[0-9_]*" suffix.
var base60float = regexp.MustCompile(`^[-+]?[0-9][0-9_]*(?::[0-5]?[0-9])+(?:\.[0-9_]*)?$`)

// isOldBool returns whether s is bool notation as defined in YAML 1.1.
//
// We continue to force strings that YAML 1.1 would interpret as booleans to be
// rendered as quotes strings so that the marshaled output valid for YAML 1.1
// parsing.
func isOldBool(s string) (result bool) {
	switch s {
	case "y", "Y", "yes", "Yes", "YES", "on", "On", "ON",
		"n", "N", "no", "No", "NO", "off", "Off", "OFF":
		return true
	default:
		return false
	}
}

// looksLikeMerge returns true if the given string is the merge indicator "<<".
//
// When encoding a scalar with this exact value, it must be quoted to prevent it
// from being interpreted as a merge indicator during decoding.
func looksLikeMerge(s string) (result bool) {
	return s == "<<"
}

func (e *encoder) stringv(tag string, in reflect.Value) {
	var style libyaml.ScalarStyle
	s := in.String()
	canUsePlain := true
	switch {
	case !utf8.ValidString(s):
		if tag == binaryTag {
			failf("explicitly tagged !!binary data must be base64-encoded")
		}
		if tag != "" {
			failf("cannot marshal invalid UTF-8 data as %s", shortTag(tag))
		}
		// It can't be encoded directly as YAML so use a binary tag
		// and encode it as base64.
		tag = binaryTag
		s = encodeBase64(s)
	case tag == "":
		// Check to see if it would resolve to a specific
		// tag when encoded unquoted. If it doesn't,
		// there's no need to quote it.
		rtag, _ := resolve("", s)
		canUsePlain = rtag == strTag &&
			!(isBase60Float(s) ||
				isOldBool(s) ||
				looksLikeMerge(s))
	}
	// Note: it's possible for user code to emit invalid YAML
	// if they explicitly specify a tag and a string containing
	// text that's incompatible with that tag.
	switch {
	case strings.Contains(s, "\n"):
		if e.flow || !shouldUseLiteralStyle(s) {
			style = libyaml.DOUBLE_QUOTED_SCALAR_STYLE
		} else {
			style = libyaml.LITERAL_SCALAR_STYLE
		}
	case canUsePlain:
		style = libyaml.PLAIN_SCALAR_STYLE
	default:
		style = libyaml.DOUBLE_QUOTED_SCALAR_STYLE
	}
	e.emitScalar(s, "", tag, style, nil, nil, nil, nil)
}

func (e *encoder) boolv(tag string, in reflect.Value) {
	var s string
	if in.Bool() {
		s = "true"
	} else {
		s = "false"
	}
	e.emitScalar(s, "", tag, libyaml.PLAIN_SCALAR_STYLE, nil, nil, nil, nil)
}

func (e *encoder) intv(tag string, in reflect.Value) {
	s := strconv.FormatInt(in.Int(), 10)
	e.emitScalar(s, "", tag, libyaml.PLAIN_SCALAR_STYLE, nil, nil, nil, nil)
}

func (e *encoder) uintv(tag string, in reflect.Value) {
	s := strconv.FormatUint(in.Uint(), 10)
	e.emitScalar(s, "", tag, libyaml.PLAIN_SCALAR_STYLE, nil, nil, nil, nil)
}

func (e *encoder) timev(tag string, in reflect.Value) {
	t := in.Interface().(time.Time)
	s := t.Format(time.RFC3339Nano)
	e.emitScalar(s, "", tag, libyaml.PLAIN_SCALAR_STYLE, nil, nil, nil, nil)
}

func (e *encoder) floatv(tag string, in reflect.Value) {
	// Issue #352: When formatting, use the precision of the underlying value
	precision := 64
	if in.Kind() == reflect.Float32 {
		precision = 32
	}

	s := strconv.FormatFloat(in.Float(), 'g', -1, precision)
	switch s {
	case "+Inf":
		s = ".inf"
	case "-Inf":
		s = "-.inf"
	case "NaN":
		s = ".nan"
	}
	e.emitScalar(s, "", tag, libyaml.PLAIN_SCALAR_STYLE, nil, nil, nil, nil)
}

func (e *encoder) nilv() {
	e.emitScalar("null", "", "", libyaml.PLAIN_SCALAR_STYLE, nil, nil, nil, nil)
}

func (e *encoder) emitScalar(value, anchor, tag string, style libyaml.ScalarStyle, head, line, foot, tail []byte) {
	// TODO Kill this function. Replace all initialize calls by their underlining Go literals.
	implicit := tag == ""
	if !implicit {
		tag = longTag(tag)
	}
	e.event = libyaml.NewScalarEvent([]byte(anchor), []byte(tag), []byte(value), implicit, implicit, style)
	e.event.HeadComment = head
	e.event.LineComment = line
	e.event.FootComment = foot
	e.event.TailComment = tail
	e.emit()
}

func (e *encoder) nodev(in reflect.Value) {
	e.node(in.Interface().(*Node), "")
}

func (e *encoder) node(node *Node, tail string) {
	// Zero nodes behave as nil.
	if node.Kind == 0 && node.IsZero() {
		e.nilv()
		return
	}

	// If the tag was not explicitly requested, and dropping it won't change the
	// implicit tag of the value, don't include it in the presentation.
	tag := node.Tag
	stag := shortTag(tag)
	var forceQuoting bool
	if tag != "" && node.Style&TaggedStyle == 0 {
		if node.Kind == ScalarNode {
			if stag == strTag && node.Style&(SingleQuotedStyle|DoubleQuotedStyle|LiteralStyle|FoldedStyle) != 0 {
				tag = ""
			} else {
				rtag, _ := resolve("", node.Value)
				if rtag == stag {
					tag = ""
				} else if stag == strTag {
					tag = ""
					forceQuoting = true
				}
			}
		} else {
			var rtag string
			switch node.Kind {
			case MappingNode:
				rtag = mapTag
			case SequenceNode:
				rtag = seqTag
			}
			if rtag == stag {
				tag = ""
			}
		}
	}

	switch node.Kind {
	case DocumentNode:
		e.event = libyaml.NewDocumentStartEvent(nil, nil, true)
		e.event.HeadComment = []byte(node.HeadComment)
		e.emit()
		for _, node := range node.Content {
			e.node(node, "")
		}
		e.event = libyaml.NewDocumentEndEvent(true)
		e.event.FootComment = []byte(node.FootComment)
		e.emit()

	case SequenceNode:
		style := libyaml.BLOCK_SEQUENCE_STYLE
		if node.Style&FlowStyle != 0 {
			style = libyaml.FLOW_SEQUENCE_STYLE
		}
		e.event = libyaml.NewSequenceStartEvent([]byte(node.Anchor), []byte(longTag(tag)), tag == "", style)
		e.event.HeadComment = []byte(node.HeadComment)
		e.emit()
		for _, node := range node.Content {
			e.node(node, "")
		}
		e.event = libyaml.NewSequenceEndEvent()
		e.event.LineComment = []byte(node.LineComment)
		e.event.FootComment = []byte(node.FootComment)
		e.emit()

	case MappingNode:
		style := libyaml.BLOCK_MAPPING_STYLE
		if node.Style&FlowStyle != 0 {
			style = libyaml.FLOW_MAPPING_STYLE
		}
		e.event = libyaml.NewMappingStartEvent([]byte(node.Anchor), []byte(longTag(tag)), tag == "", style)
		e.event.TailComment = []byte(tail)
		e.event.HeadComment = []byte(node.HeadComment)
		e.emit()

		// The tail logic below moves the foot comment of prior keys to the following key,
		// since the value for each key may be a nested structure and the foot needs to be
		// processed only the entirety of the value is streamed. The last tail is processed
		// with the mapping end event.
		var tail string
		for i := 0; i+1 < len(node.Content); i += 2 {
			k := node.Content[i]
			foot := k.FootComment
			if foot != "" {
				kopy := *k
				kopy.FootComment = ""
				k = &kopy
			}
			e.node(k, tail)
			tail = foot

			v := node.Content[i+1]
			e.node(v, "")
		}

		e.event = libyaml.NewMappingEndEvent()
		e.event.TailComment = []byte(tail)
		e.event.LineComment = []byte(node.LineComment)
		e.event.FootComment = []byte(node.FootComment)
		e.emit()

	case AliasNode:
		e.event = libyaml.NewAliasEvent([]byte(node.Value))
		e.event.HeadComment = []byte(node.HeadComment)
		e.event.LineComment = []byte(node.LineComment)
		e.event.FootComment = []byte(node.FootComment)
		e.emit()

	case ScalarNode:
		value := node.Value
		if !utf8.ValidString(value) {
			if stag == binaryTag {
				failf("explicitly tagged !!binary data must be base64-encoded")
			}
			if stag != "" {
				failf("cannot marshal invalid UTF-8 data as %s", stag)
			}
			// It can't be encoded directly as YAML so use a binary tag
			// and encode it as base64.
			tag = binaryTag
			value = encodeBase64(value)
		}

		style := libyaml.PLAIN_SCALAR_STYLE
		switch {
		case node.Style&DoubleQuotedStyle != 0:
			style = libyaml.DOUBLE_QUOTED_SCALAR_STYLE
		case node.Style&SingleQuotedStyle != 0:
			style = libyaml.SINGLE_QUOTED_SCALAR_STYLE
		case node.Style&LiteralStyle != 0:
			style = libyaml.LITERAL_SCALAR_STYLE
		case node.Style&FoldedStyle != 0:
			style = libyaml.FOLDED_SCALAR_STYLE
		case strings.Contains(value, "\n"):
			style = libyaml.LITERAL_SCALAR_STYLE
		case forceQuoting:
			style = libyaml.DOUBLE_QUOTED_SCALAR_STYLE
		}

		e.emitScalar(value, node.Anchor, tag, style, []byte(node.HeadComment), []byte(node.LineComment), []byte(node.FootComment), []byte(tail))
	default:
		failf("cannot encode node with unknown kind %d", node.Kind)
	}
}
