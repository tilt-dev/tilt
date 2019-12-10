package config

import (
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/windmilleng/tilt/internal/tiltfile/starkit"
	"github.com/windmilleng/tilt/internal/tiltfile/value"
	"github.com/windmilleng/tilt/pkg/model"
)

func TestSetResources(t *testing.T) {
	for _, tc := range []struct {
		name              string
		callFlagsParse    bool
		argsResources     []model.ManifestName
		tiltfileResources []model.ManifestName
		expectedResources []model.ManifestName
	}{
		{"neither", false, nil, nil, []model.ManifestName{"a", "b"}},
		{"neither, with config.parse", true, nil, nil, []model.ManifestName{"a", "b"}},
		{"args only", false, []model.ManifestName{"a"}, nil, []model.ManifestName{"a"}},
		{"args only, with config.parse", true, []model.ManifestName{"a"}, nil, []model.ManifestName{"a", "b"}},
		{"tiltfile only", false, nil, []model.ManifestName{"b"}, []model.ManifestName{"b"}},
		{"tiltfile only, with config.parse", true, nil, []model.ManifestName{"b"}, []model.ManifestName{"b"}},
		{"both", false, []model.ManifestName{"a"}, []model.ManifestName{"b"}, []model.ManifestName{"b"}},
		{"both, with config.parse", true, []model.ManifestName{"a"}, []model.ManifestName{"b"}, []model.ManifestName{"b"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := NewFixture(t)

			setResources := ""
			if len(tc.tiltfileResources) > 0 {
				var rs []string
				for _, mn := range tc.tiltfileResources {
					rs = append(rs, fmt.Sprintf("'%s'", mn))
				}
				setResources = fmt.Sprintf("config.set_enabled_resources([%s])", strings.Join(rs, ", "))
			}

			flagsParse := ""
			if tc.callFlagsParse {
				flagsParse = "config.parse()"
			}

			tiltfile := fmt.Sprintf("%s\n%s\n", setResources, flagsParse)

			f.File("Tiltfile", tiltfile)

			result, err := f.ExecFile("Tiltfile")
			require.NoError(t, err)

			var args []string
			for _, a := range tc.argsResources {
				args = append(args, string(a))
			}

			manifests := []model.Manifest{{Name: "a"}, {Name: "b"}}
			actual, err := MustState(result).EnabledResources(args, manifests)
			require.NoError(t, err)

			expectedResourcesByName := make(map[model.ManifestName]bool)
			for _, er := range tc.expectedResources {
				expectedResourcesByName[er] = true
			}
			var expected []model.Manifest
			for _, m := range manifests {
				if expectedResourcesByName[m.Name] {
					expected = append(expected, m)
				}
			}
			require.Equal(t, expected, actual)
		})
	}
}

func TestParsePositional(t *testing.T) {
	foo := strings.Split("united states canada mexico panama haiti jamaica peru", " ")

	f := NewFixture(t, foo...)
	f.File("Tiltfile", `
config.define_string_list('foo', args=True)
cfg = config.parse()
print(cfg['foo'])
`)

	_, err := f.ExecFile("Tiltfile")
	require.NoError(t, err)

	require.Contains(t, f.PrintOutput(), value.StringSliceToList(foo).String())
}

func TestParseKeyword(t *testing.T) {
	foo := strings.Split("republic dominican cuba caribbean greenland el salvador too", " ")
	var args []string
	for _, s := range foo {
		args = append(args, []string{"-foo", s}...)
	}

	f := NewFixture(t, args...)
	f.File("Tiltfile", `
config.define_string_list('foo')
cfg = config.parse()
print(cfg['foo'])
`)

	_, err := f.ExecFile("Tiltfile")
	require.NoError(t, err)

	require.Contains(t, f.PrintOutput(), value.StringSliceToList(foo).String())
}

func TestParsePositionalAndMultipleInterspersedKeyword(t *testing.T) {
	args := []string{"-bar", "puerto rico", "-baz", "colombia", "-bar", "venezuela", "-baz", "honduras", "-baz", "guyana", "and", "still"}
	f := NewFixture(t, args...)

	f.File("Tiltfile", `
config.define_string_list('foo', args=True)
config.define_string_list('bar')
config.define_string_list('baz')
cfg = config.parse()
print("foo:", cfg['foo'])
print("bar:", cfg['bar'])
print("baz:", cfg['baz'])
`)

	_, err := f.ExecFile("Tiltfile")
	require.NoError(t, err)

	require.Contains(t, f.PrintOutput(), `foo: ["and", "still"]`)
	require.Contains(t, f.PrintOutput(), `bar: ["puerto rico", "venezuela"]`)
	require.Contains(t, f.PrintOutput(), `baz: ["colombia", "honduras", "guyana"]`)
}

func TestMultiplePositionalDefs(t *testing.T) {
	f := NewFixture(t)
	f.File("Tiltfile", `
config.define_string_list('foo', args=True)
config.define_string_list('bar', args=True)
`)

	_, err := f.ExecFile("Tiltfile")
	require.Error(t, err)
	require.Equal(t, "both bar and foo are defined as positional args", err.Error())
}

func TestMultipleArgsSameName(t *testing.T) {
	f := NewFixture(t)
	f.File("Tiltfile", `
config.define_string_list('foo')
config.define_string_list('foo')
`)

	_, err := f.ExecFile("Tiltfile")
	require.Error(t, err)
	require.Equal(t, "foo defined multiple times", err.Error())
}

func TestUndefinedArg(t *testing.T) {
	f := NewFixture(t, "-bar", "hello")
	f.File("Tiltfile", `
config.define_string_list('foo')
config.parse()
`)

	_, err := f.ExecFile("Tiltfile")
	require.Error(t, err)
	require.Equal(t, "flag provided but not defined: -bar", err.Error())
}

func TestUnprovidedKeywordArg(t *testing.T) {
	f := NewFixture(t)
	f.File("Tiltfile", `
config.define_string_list('foo')
cfg = config.parse()
print("foo:",cfg['foo'])
`)

	_, err := f.ExecFile("Tiltfile")
	require.NoError(t, err)
	require.Contains(t, f.PrintOutput(), "foo: []")
}

func TestUnprovidedPositionalArg(t *testing.T) {
	f := NewFixture(t)
	f.File("Tiltfile", `
config.define_string_list('foo', args=True)
cfg = config.parse()
print("foo:",cfg['foo'])
`)

	_, err := f.ExecFile("Tiltfile")
	require.NoError(t, err)
	require.Contains(t, f.PrintOutput(), "foo: []")
}

func TestProvidedButUnexpectedPositionalArgs(t *testing.T) {
	f := NewFixture(t, "do", "re", "mi")
	f.File("Tiltfile", `
cfg = config.parse()
`)

	_, err := f.ExecFile("Tiltfile")
	require.Error(t, err)
	require.Equal(t, "positional args were specified, but none were expected (no flag defined with args=True)", err.Error())
}

func TestUsage(t *testing.T) {
	f := NewFixture(t, "-bar", "hello")
	f.File("Tiltfile", `
config.define_string_list('foo', usage='what can I foo for you today?')
config.parse()
`)

	_, err := f.ExecFile("Tiltfile")
	require.Error(t, err)
	require.Contains(t, err.Error(), "flag provided but not defined: -bar")
	require.Contains(t, f.PrintOutput(), "Usage:")
	require.Contains(t, f.PrintOutput(), "what can I foo for you today")
}

// i.e., tilt up foo bar gets you resources foo and bar
func TestDefaultTiltBehavior(t *testing.T) {
	f := NewFixture(t, "foo", "bar")
	f.File("Tiltfile", `
config.define_string_list('resources', usage='which resources to load in Tilt', args=True)
config.set_enabled_resources(config.parse()['resources'])
`)

	result, err := f.ExecFile("Tiltfile")
	require.NoError(t, err)

	manifests := []model.Manifest{{Name: "foo"}, {Name: "bar"}, {Name: "baz"}}
	actual, err := MustState(result).EnabledResources([]string{"foo", "bar"}, manifests)
	require.NoError(t, err)
	require.Equal(t, manifests[:2], actual)

}

func NewFixture(tb testing.TB, args ...string) *starkit.Fixture {
	return starkit.NewFixture(tb, NewExtension(args))
}
