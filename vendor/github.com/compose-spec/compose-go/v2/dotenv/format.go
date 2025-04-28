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

package dotenv

import (
	"fmt"
	"io"
)

const DotEnv = ".env"

var formats = map[string]Parser{
	DotEnv: func(r io.Reader, filename string, vars map[string]string, lookup func(key string) (string, bool)) error {
		err := parseWithLookup(r, vars, lookup)
		if err != nil {
			return fmt.Errorf("failed to read %s: %w", filename, err)
		}
		return nil
	},
}

type Parser func(r io.Reader, filename string, vars map[string]string, lookup func(key string) (string, bool)) error

func RegisterFormat(format string, p Parser) {
	formats[format] = p
}

func ParseWithFormat(r io.Reader, filename string, vars map[string]string, resolve LookupFn, format string) error {
	if format == "" {
		format = DotEnv
	}
	fn, ok := formats[format]
	if !ok {
		return fmt.Errorf("unsupported env_file format %q", format)
	}
	return fn(r, filename, vars, resolve)
}
