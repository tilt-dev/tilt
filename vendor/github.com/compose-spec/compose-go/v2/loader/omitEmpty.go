/*
   Copyright 2020 The Compose Specification Authors.

   Licensed under the Apache License, Version 2.0 (the "License");
   you may not use this file except in compliance with the License.
   You may obtain a copy of the License at

       http://www.apache.org/licenses/LICENSE-2.0

   Unless required by applicable law or agreed to in writing, software
   distributed under the License is distributed on an "AS IS" BASIS,
   WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
   See the License for the specific language governing permissions and
   limitations under the License.
*/

package loader

import "github.com/compose-spec/compose-go/v2/tree"

var omitempty = []tree.Path{
	"services.*.dns",
}

// OmitEmpty removes empty attributes which are irrelevant when unset
func OmitEmpty(yaml map[string]any) map[string]any {
	cleaned := omitEmpty(yaml, tree.NewPath())
	return cleaned.(map[string]any)
}

func omitEmpty(data any, p tree.Path) any {
	switch v := data.(type) {
	case map[string]any:
		for k, e := range v {
			if isEmpty(e) && mustOmit(p) {
				delete(v, k)
				continue
			}

			v[k] = omitEmpty(e, p.Next(k))
		}
		return v
	case []any:
		var c []any
		for _, e := range v {
			if isEmpty(e) && mustOmit(p) {
				continue
			}

			c = append(c, omitEmpty(e, p.Next("[]")))
		}
		return c
	default:
		return data
	}
}

func mustOmit(p tree.Path) bool {
	for _, pattern := range omitempty {
		if p.Matches(pattern) {
			return true
		}
	}
	return false
}

func isEmpty(e any) bool {
	if e == nil {
		return true
	}
	if v, ok := e.(string); ok && v == "" {
		return true
	}
	return false
}
