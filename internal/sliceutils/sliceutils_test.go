package sliceutils

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAppendWithoutDupesNoDupes(t *testing.T) {
	a := []string{"a", "b"}
	b := []string{"c", "d"}
	observed := AppendWithoutDupes(a, b...)
	expected := []string{"a", "b", "c", "d"}
	assert.Equal(t, expected, observed)
}

func TestAppendWithoutDupesHasADupe(t *testing.T) {
	a := []string{"a", "b"}
	b := []string{"c", "b"}
	observed := AppendWithoutDupes(a, b...)
	expected := []string{"a", "b", "c"}
	assert.Equal(t, expected, observed)
}

func TestAppendWithoutDupesEmptyA(t *testing.T) {
	a := []string{}
	b := []string{"c", "b"}
	observed := AppendWithoutDupes(a, b...)
	expected := []string{"c", "b"}
	assert.Equal(t, expected, observed)
}

func TestAppendWithoutDupesEmptyB(t *testing.T) {
	a := []string{"a", "b"}
	b := []string{}
	observed := AppendWithoutDupes(a, b...)
	expected := []string{"a", "b"}
	assert.Equal(t, expected, observed)
}

func TestUnescapeAndSplitNoEscape(t *testing.T) {
	parts, err := UnescapeAndSplit("a:b:c", NewEscapeSplitOptions())
	require.NoError(t, err)
	require.Equal(t, []string{"a", "b", "c"}, parts)
}

func TestUnescapeAndSplitEscapedDelimiter(t *testing.T) {
	parts, err := UnescapeAndSplit(`a\:d:b:c`, NewEscapeSplitOptions())
	require.NoError(t, err)
	require.Equal(t, []string{"a:d", "b", "c"}, parts)
}

func TestUnescapeAndSplitEscapedEscapeChar(t *testing.T) {
	parts, err := UnescapeAndSplit(`a\\d:b:c`, NewEscapeSplitOptions())
	require.NoError(t, err)
	require.Equal(t, []string{`a\d`, "b", "c"}, parts)
}

func TestUnescapeAndSplitInvalidEscape(t *testing.T) {
	_, err := UnescapeAndSplit(`a\d:b:c`, NewEscapeSplitOptions())
	require.Error(t, err)
	require.Contains(t, err.Error(), `invalid escape sequence '\d' in 'a\d:b'`)
}

func TestEscapeAndJoinNoEscape(t *testing.T) {
	s := EscapeAndJoin([]string{"a", "b", "c"}, NewEscapeSplitOptions())
	require.Equal(t, s, "a:b:c", s)
}

func TestEscapeAndJoinEscapeDelimiter(t *testing.T) {
	s := EscapeAndJoin([]string{"a:d", "b", "c"}, NewEscapeSplitOptions())
	require.Equal(t, s, `a\:d:b:c`, s)
}

func TestEscapeAndJoinEscapeEscapeChar(t *testing.T) {
	s := EscapeAndJoin([]string{`a\d`, "b", "c"}, NewEscapeSplitOptions())
	require.Equal(t, s, `a\\d:b:c`, s)
}

func TestUnescapeAndSplitRoundtrip(t *testing.T) {
	expected := `a\\d:\:b\:::c\:e`
	opts := NewEscapeSplitOptions()
	parts, err := UnescapeAndSplit(expected, opts)
	require.NoError(t, err)
	observed := EscapeAndJoin(parts, opts)
	require.Equal(t, expected, observed)
}
