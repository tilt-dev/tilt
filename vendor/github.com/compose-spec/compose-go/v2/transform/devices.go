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

package transform

import (
	"fmt"

	"github.com/compose-spec/compose-go/v2/tree"
)

func deviceRequestDefaults(data any, p tree.Path, _ bool) (any, error) {
	v, ok := data.(map[string]any)
	if !ok {
		return data, fmt.Errorf("%s: invalid type %T for device request", p, v)
	}
	_, hasCount := v["count"]
	_, hasIDs := v["device_ids"]
	if !hasCount && !hasIDs {
		v["count"] = "all"
	}
	return v, nil
}
