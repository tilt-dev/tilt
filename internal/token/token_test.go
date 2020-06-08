package token

import (
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/tilt-dev/wmclient/pkg/dirs"

	"github.com/tilt-dev/tilt/internal/testutils/tempdir"
)

func TestGetOrCreateToken(t *testing.T) {
	f := newFixture(t)
	t1, err := GetOrCreateToken(f.dir)
	if err != nil {
		t.Fatal(err)
	}
	t2, err := GetOrCreateToken(f.dir)
	if err != nil {
		t.Fatal(err)
	}
	// The token returned in the second GetOrCreateToken call should be the same as the one
	// that was created in the first call.
	// This test thus demonstrates both that a token is created if it doesn't exist
	// and that a token can be read if it does exist
	require.Equal(f.t, t1, t2)
}

type fixture struct {
	*tempdir.TempDirFixture
	t   *testing.T
	dir *dirs.WindmillDir
}

func newFixture(t *testing.T) *fixture {
	f := tempdir.NewTempDirFixture(t)
	temp := dirs.NewWindmillDirAt(f.Path())
	return &fixture{
		TempDirFixture: f,
		t:              t,
		dir:            temp,
	}
}
