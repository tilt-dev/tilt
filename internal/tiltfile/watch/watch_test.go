package watch

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/tilt-dev/tilt/internal/tiltfile/starkit"
	"github.com/tilt-dev/tilt/pkg/model"
)

func TestBasic(t *testing.T) {
	f := NewFixture(t)
	f.File("Tiltfile", `
watch_settings(ignore=['foo'])
`)
	result, err := f.ExecFile("Tiltfile")
	require.NoError(t, err)
	require.Equal(t, model.WatchSettings{
		Ignores: []model.Dockerignore{
			{
				LocalPath: f.Path(),
				Patterns:  []string{"foo"},
				Source:    "watch_settings()",
			},
		},
	}, MustState(result))
}

func TestLoaded(t *testing.T) {
	f := NewFixture(t)
	f.File("foo/Tiltfile", `
watch_settings(ignore=['bar'])
x = 1
`)
	f.File("Tiltfile", `
load('./foo/Tiltfile', 'x')
`)
	result, err := f.ExecFile("Tiltfile")
	require.NoError(t, err)
	require.Equal(t, model.WatchSettings{
		Ignores: []model.Dockerignore{
			{
				LocalPath: f.JoinPath("foo"),
				Patterns:  []string{"bar"},
				Source:    "watch_settings()",
			},
		},
	}, MustState(result))
}

func NewFixture(tb testing.TB) *starkit.Fixture {
	return starkit.NewFixture(tb, NewExtension())
}
