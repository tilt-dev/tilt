package io

import (
	"fmt"

	"go.starlark.net/starlark"

	"github.com/windmilleng/tilt/internal/tiltfile/value"
)

type Blob struct {
	Text   string
	Source string
}

var _ starlark.Value = Blob{}

func NewBlob(text string, source string) Blob {
	return Blob{Text: text, Source: source}
}

func (b Blob) ImplicitString() string {
	return b.Text
}

func (b Blob) String() string {
	return b.Text
}

func (b Blob) Type() string {
	return "blob"
}

func (b Blob) Freeze() {}

func (b Blob) Truth() starlark.Bool {
	return len(b.Text) > 0
}

func (b Blob) Hash() (uint32, error) {
	return 0, fmt.Errorf("unhashable type: blob")
}

var _ value.ImplicitStringer = Blob{}
