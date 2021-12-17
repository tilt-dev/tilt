package value

import (
	"fmt"

	"go.starlark.net/starlark"
)

type LocalPath struct {
	t     *starlark.Thread
	Value string
}

func NewLocalPathUnpacker(t *starlark.Thread) LocalPath {
	return LocalPath{
		t: t,
	}
}

func (p *LocalPath) Unpack(v starlark.Value) error {
	str, err := ValueToAbsPath(p.t, v)
	if err != nil {
		return err
	}

	p.Value = str
	return nil
}

type LocalPathList struct {
	t     *starlark.Thread
	Value []string
}

func NewLocalPathListUnpacker(t *starlark.Thread) LocalPathList {
	return LocalPathList{
		t: t,
	}
}

func (p *LocalPathList) Unpack(v starlark.Value) error {
	_, ok := AsString(v)
	if ok {
		str, err := ValueToAbsPath(p.t, v)
		if err != nil {
			return err
		}
		p.Value = []string{str}
		return nil
	}

	var iter starlark.Iterator
	switch x := v.(type) {
	case *starlark.List:
		iter = x.Iterate()
	case starlark.Tuple:
		iter = x.Iterate()
	case starlark.NoneType:
		return nil
	default:
		return fmt.Errorf("value should be a string or List or Tuple of strings, but is of type %s", v.Type())

	}

	defer iter.Done()

	values := []string{}
	var item starlark.Value
	for iter.Next(&item) {
		str, err := ValueToAbsPath(p.t, item)
		if err != nil {
			return fmt.Errorf("unpacking list item at index %d: %v", len(values), err)
		}
		values = append(values, str)
	}
	p.Value = values
	return nil
}
