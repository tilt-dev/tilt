package tempdir

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"

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

func TestSanitizeForFilename(t *testing.T) {
	for _, tc := range []struct {
		input          string
		expectedOutput string
	}{
		{"foobar", "foobar"},
		{"a>=b", "a__b"},
		{"1/2 power", "1_2_power"},
		{"cat:dog::mouse:cat", "cat_dog__mouse_cat"},
	} {
		t.Run(fmt.Sprintf("escape %s", tc.input), func(t *testing.T) {
			observed := SanitizeFileName(tc.input)
			require.Equal(t, tc.expectedOutput, observed)
		})
	}
}
