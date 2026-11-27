package cli

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
)

type fakeRuneReader struct {
	runes []rune
	err   error
	reads int
}

func (f *fakeRuneReader) ReadRune() (rune, error) {
	f.reads++
	if len(f.runes) == 0 {
		return 0, f.err
	}
	r := f.runes[0]
	f.runes = f.runes[1:]
	return r, nil
}

// On Windows, go-tty reads a key release or a focus change as rune 0 (#6024).
func TestReadKeySkipsEventsThatAreNotKeys(t *testing.T) {
	r := &fakeRuneReader{runes: []rune{0, 0, 'q'}}

	require.NoError(t, readKey(r))
	require.Equal(t, 3, r.reads)
}

func TestReadKeyReturnsTheReadError(t *testing.T) {
	r := &fakeRuneReader{runes: []rune{0}, err: errors.New("closed")}

	require.EqualError(t, readKey(r), "closed")
}
