package encoding

import (
	"fmt"

	"github.com/pkg/errors"
	"go.starlark.net/starlark"
)

func convertStructuredDataToStarlark(j interface{}) (starlark.Value, error) {
	switch j := j.(type) {
	case bool:
		return starlark.Bool(j), nil
	case string:
		return starlark.String(j), nil
	case float64:
		return starlark.Float(j), nil
	case []interface{}:
		listOfValues := []starlark.Value{}

		for _, v := range j {
			convertedValue, err := convertStructuredDataToStarlark(v)
			if err != nil {
				return nil, err
			}
			listOfValues = append(listOfValues, convertedValue)
		}

		return starlark.NewList(listOfValues), nil
	case map[string]interface{}:
		mapOfValues := &starlark.Dict{}

		for k, v := range j {
			convertedValue, err := convertStructuredDataToStarlark(v)
			if err != nil {
				return nil, err
			}

			err = mapOfValues.SetKey(starlark.String(k), convertedValue)
			if err != nil {
				return nil, err
			}
		}

		return mapOfValues, nil
	case nil:
		return starlark.None, nil
	}

	return nil, errors.New(fmt.Sprintf("Unable to convert to starlark value, unexpected type %T", j))
}
