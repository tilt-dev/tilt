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

package types

import (
	"fmt"
	"strconv"
)

type NanoCPUs float32

func (n *NanoCPUs) DecodeMapstructure(a any) error {
	switch v := a.(type) {
	case string:
		f, err := strconv.ParseFloat(v, 64)
		if err != nil {
			return err
		}
		*n = NanoCPUs(f)
	case int:
		*n = NanoCPUs(v)
	case float32:
		*n = NanoCPUs(v)
	case float64:
		*n = NanoCPUs(v)
	default:
		return fmt.Errorf("unexpected value type %T for cpus", v)
	}
	return nil
}

func (n *NanoCPUs) Value() float32 {
	return float32(*n)
}
