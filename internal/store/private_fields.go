package store

/*!
MIT License

Copyright (c) 2016 json-iterator

Permission is hereby granted, free of charge, to any person obtaining a copy
of this software and associated documentation files (the "Software"), to deal
in the Software without restriction, including without limitation the rights
to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
copies of the Software, and to permit persons to whom the Software is
furnished to do so, subject to the following conditions:

The above copyright notice and this permission notice shall be included in all
copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
SOFTWARE.
*/

// Copied from
// https://github.com/json-iterator/go/blob/master/extra/privat_fields.go
// and made possible to re-use in a particular api config

import (
	"strings"
	"unicode"

	jsoniter "github.com/json-iterator/go"
)

type privateFieldsExtension struct {
	jsoniter.DummyExtension
}

func (extension *privateFieldsExtension) UpdateStructDescriptor(structDescriptor *jsoniter.StructDescriptor) {
	for _, binding := range structDescriptor.Fields {
		isPrivate := unicode.IsLower(rune(binding.Field.Name()[0]))
		if isPrivate {
			tag, hastag := binding.Field.Tag().Lookup("json")
			if !hastag {
				binding.FromNames = []string{binding.Field.Name()}
				binding.ToNames = []string{binding.Field.Name()}
				continue
			}
			tagParts := strings.Split(tag, ",")
			names := calcFieldNames(binding.Field.Name(), tagParts[0], tag)
			binding.FromNames = names
			binding.ToNames = names
		}
	}
}

func calcFieldNames(originalFieldName string, tagProvidedFieldName string, wholeTag string) []string {
	// ignore?
	if wholeTag == "-" {
		return []string{}
	}
	// rename?
	var fieldNames []string
	if tagProvidedFieldName == "" {
		fieldNames = []string{originalFieldName}
	} else {
		fieldNames = []string{tagProvidedFieldName}
	}
	// private?
	isNotExported := unicode.IsLower(rune(originalFieldName[0]))
	if isNotExported {
		fieldNames = []string{}
	}
	return fieldNames
}
