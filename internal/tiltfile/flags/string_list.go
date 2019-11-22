package flags

import (
	"flag"
	"strings"

	"go.starlark.net/starlark"

	"github.com/windmilleng/tilt/internal/tiltfile/value"
)

type stringList struct {
	f *Strings
}

func (s *stringList) flag() flag.Value {
	s.f = &Strings{}
	return s.f
}

func (s *stringList) starlark() starlark.Value {
	return value.StringSliceToList(s.f.Values)
}

func (s *stringList) setFromArgs(strs []string) {
	s.f = &Strings{Values: strs}
}

// Strings is a `flag.Value` for `string` arguments. (from https://github.com/sgreben/flagvar/blob/master/string.go)
type Strings struct {
	Values []string
}

// Set is flag.Value.Set
func (fv *Strings) Set(v string) error {
	fv.Values = append(fv.Values, v)
	return nil
}

func (fv *Strings) String() string {
	return strings.Join(fv.Values, ",")
}
