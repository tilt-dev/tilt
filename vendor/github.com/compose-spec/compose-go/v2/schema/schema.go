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

package schema

import (
	// Enable support for embedded static resources
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/santhosh-tekuri/jsonschema/v6"
	"github.com/santhosh-tekuri/jsonschema/v6/kind"
	"golang.org/x/text/language"
	"golang.org/x/text/message"
)

func durationFormatChecker(input any) error {
	value, ok := input.(string)
	if !ok {
		return fmt.Errorf("expected string")
	}
	_, err := time.ParseDuration(value)
	return err
}

// Schema is the compose-spec JSON schema
//
//go:embed compose-spec.json
var Schema string

// Validate uses the jsonschema to validate the configuration
func Validate(config map[string]interface{}) error {
	compiler := jsonschema.NewCompiler()
	shema, err := jsonschema.UnmarshalJSON(strings.NewReader(Schema))
	if err != nil {
		return err
	}
	err = compiler.AddResource("compose-spec.json", shema)
	if err != nil {
		return err
	}
	compiler.RegisterFormat(&jsonschema.Format{
		Name:     "duration",
		Validate: durationFormatChecker,
	})
	schema := compiler.MustCompile("compose-spec.json")

	// santhosh-tekuri doesn't allow derived types
	// see https://github.com/santhosh-tekuri/jsonschema/pull/240
	marshaled, err := json.Marshal(config)
	if err != nil {
		return err
	}

	var raw map[string]interface{}
	err = json.Unmarshal(marshaled, &raw)
	if err != nil {
		return err
	}

	err = schema.Validate(raw)
	var verr *jsonschema.ValidationError
	if ok := errors.As(err, &verr); ok {
		return validationError{getMostSpecificError(verr)}
	}
	return err
}

type validationError struct {
	err *jsonschema.ValidationError
}

func (e validationError) Error() string {
	path := strings.Join(e.err.InstanceLocation, ".")
	p := message.NewPrinter(language.English)
	switch k := e.err.ErrorKind.(type) {
	case *kind.Type:
		return fmt.Sprintf("%s must be a %s", path, humanReadableType(k.Want...))
	case *kind.Minimum:
		return fmt.Sprintf("%s must be greater than or equal to %s", path, k.Want.Num())
	case *kind.Maximum:
		return fmt.Sprintf("%s must be less than or equal to %s", path, k.Want.Num())
	}
	return fmt.Sprintf("%s %s", path, e.err.ErrorKind.LocalizedString(p))
}

func humanReadableType(want ...string) string {
	if len(want) == 1 {
		switch want[0] {
		case "object":
			return "mapping"
		default:
			return want[0]
		}
	}

	for i, s := range want {
		want[i] = humanReadableType(s)
	}

	slices.Sort(want)
	return fmt.Sprintf(
		"%s or %s",
		strings.Join(want[0:len(want)-1], ", "),
		want[len(want)-1],
	)
}

func getMostSpecificError(err *jsonschema.ValidationError) *jsonschema.ValidationError {
	var mostSpecificError *jsonschema.ValidationError
	if len(err.Causes) == 0 {
		return err
	}
	for _, cause := range err.Causes {
		cause = getMostSpecificError(cause)
		if specificity(cause) > specificity(mostSpecificError) {
			mostSpecificError = cause
		}
	}
	return mostSpecificError
}

func specificity(err *jsonschema.ValidationError) int {
	if err == nil {
		return -1
	}
	if _, ok := err.ErrorKind.(*kind.AdditionalProperties); ok {
		return len(err.InstanceLocation) + 1
	}
	return len(err.InstanceLocation)
}
