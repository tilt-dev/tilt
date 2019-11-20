package flags

import (
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/windmilleng/tilt/internal/tiltfile/starkit"
	"github.com/windmilleng/tilt/pkg/model"
)

func TestSetResources(t *testing.T) {
	for _, tc := range []struct {
		name              string
		argsResources     []model.ManifestName
		tiltfileResources []model.ManifestName
		expectedResources []model.ManifestName
	}{
		{"neither", nil, nil, []model.ManifestName(nil)},
		{"args only", []model.ManifestName{"a"}, nil, []model.ManifestName{"a"}},
		{"tiltfile only", nil, []model.ManifestName{"b"}, []model.ManifestName{"b"}},
		{"both", []model.ManifestName{"a"}, []model.ManifestName{"b"}, []model.ManifestName{"b"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := NewFixture(t)

			setResources := ""
			if len(tc.tiltfileResources) > 0 {
				var rs []string
				for _, mn := range tc.tiltfileResources {
					rs = append(rs, fmt.Sprintf("'%s'", mn))
				}
				setResources = fmt.Sprintf("flags.set_resources([%s])", strings.Join(rs, ", "))
			}
			f.File("Tiltfile", setResources)

			result, err := f.ExecFile("Tiltfile")
			require.NoError(t, err)

			actual := MustState(result).Resources(tc.argsResources)
			require.Equal(t, tc.expectedResources, actual)
		})
	}
}

func NewFixture(tb testing.TB) *starkit.Fixture {
	return starkit.NewFixture(tb, NewExtension())
}
