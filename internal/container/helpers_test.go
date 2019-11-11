package container

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestFamiliarString(t *testing.T) {
	s := MustParseSelector("golang")
	assert.Equal(t, "golang", FamiliarString(s))
	assert.Equal(t, "golang", FamiliarString(s.AsNamedOnly()))
}
