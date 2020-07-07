package value

import (
	"fmt"
	"time"

	"go.starlark.net/starlark"
)

// Parse duration constants from starlark.
type Duration time.Duration

func (d Duration) IsZero() bool {
	return d.AsDuration() == 0
}

func (d Duration) AsDuration() time.Duration {
	return time.Duration(d)
}

func (d *Duration) Unpack(v starlark.Value) error {
	s, ok := starlark.AsString(v)
	if !ok {
		return fmt.Errorf("Expected string. Got: %s", v.Type())
	}

	dur, err := time.ParseDuration(s)
	if err != nil {
		return err
	}

	*d = Duration(dur)
	return nil
}
