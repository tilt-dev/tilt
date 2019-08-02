package engine

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestFormatChangedFileListTruncates(t *testing.T) {
	actual := formatFileChangeList([]string{"a", "b", "c", "d", "e", "f"})
	expected := "[a b c d e f]"
	require.Equal(t, expected, actual)
}

func TestFormatChangedFileListMakesRelative(t *testing.T) {
	wd, err := os.Getwd()
	require.NoError(t, err)
	actual := formatFileChangeList([]string{filepath.Join(wd, "foo"), "/bar/baz"})
	expected := "[foo /bar/baz]"
	require.Equal(t, expected, actual)
}
