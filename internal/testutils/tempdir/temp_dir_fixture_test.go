package tempdir

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNestedDirs(t *testing.T) {
	t.Run("inner", func(t *testing.T) {
		f := NewTempDirFixture(t)
		defer f.TearDown()

		assert.Contains(t, f.Path(), "inner")
		assert.Contains(t, f.Path(), "NestedDirs")
	})
}
