package tiltfile

import (
	"fmt"

	"go.starlark.net/starlark"
)

type blob struct {
	text   string
	source string
}

var _ starlark.Value = &blob{}

func newBlob(text string, source string) *blob {
	return &blob{text: text, source: source}
}

func (b *blob) String() string {
	return b.text
}

func (b *blob) Type() string {
	return "blob"
}

func (b *blob) Freeze() {}

func (b *blob) Truth() starlark.Bool {
	return len(b.text) > 0
}

func (b *blob) Hash() (uint32, error) {
	return 0, fmt.Errorf("unhashable type: blob")
}
