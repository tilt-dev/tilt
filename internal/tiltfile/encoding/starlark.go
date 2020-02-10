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

func convertStarlarkToStructuredData(v starlark.Value) (interface{}, error) {
	switch v := v.(type) {
	case starlark.Bool:
		return bool(v), nil
	case starlark.String:
		return v.GoString(), nil
	case starlark.Int:
		return v.BigInt().Int64(), nil
	case starlark.Float:
		return float64(v), nil
	case *starlark.List:
		var ret []interface{}

		it := v.Iterate()
		defer it.Done()
		var e starlark.Value
		for it.Next(&e) {
			ee, err := convertStarlarkToStructuredData(e)
			if err != nil {
				return nil, err
			}
			ret = append(ret, ee)
		}
		return ret, nil
	case *starlark.Dict:
		ret := make(map[string]interface{})
		for _, t := range v.Items() {
			key := t.Index(0)
			kk, err := convertStarlarkToStructuredData(key)
			if err != nil {
				return nil, err
			}

			s, ok := kk.(string)
			if !ok {
				return nil, fmt.Errorf("only string keys are supported in maps. found key '%s' of type %T", key.String(), kk)
			}

			val := t.Index(1)
			vv, err := convertStarlarkToStructuredData(val)
			if err != nil {
				return nil, err
			}

			ret[s] = vv
		}

		return ret, nil
	case starlark.NoneType:
		return nil, nil
	}

	return nil, fmt.Errorf("unable to convert from starlark value, unsupported type %T", v)
}
