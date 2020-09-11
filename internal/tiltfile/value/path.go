package value

import (
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
