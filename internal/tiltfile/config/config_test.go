package config

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/windmilleng/tilt/internal/tiltfile/include"
	"github.com/windmilleng/tilt/internal/tiltfile/io"
	"github.com/windmilleng/tilt/internal/tiltfile/starkit"
	"github.com/windmilleng/tilt/internal/tiltfile/value"
	"github.com/windmilleng/tilt/pkg/model"
)

func TestSetResources(t *testing.T) {
	for _, tc := range []struct {
		name              string
		callConfigParse   bool
		args              []string
		tiltfileResources []model.ManifestName
		expectedResources []model.ManifestName
	}{
		{"neither", false, nil, nil, []model.ManifestName{"a", "b"}},
		{"neither, with config.parse", true, nil, nil, []model.ManifestName{"a", "b"}},
		{"args only", false, []string{"a"}, nil, []model.ManifestName{"a"}},
		{"args only, with config.parse", true, []string{"a"}, nil, []model.ManifestName{"a", "b"}},
		{"tiltfile only", false, nil, []model.ManifestName{"b"}, []model.ManifestName{"b"}},
		{"tiltfile only, with config.parse", true, nil, []model.ManifestName{"b"}, []model.ManifestName{"b"}},
		{"both", false, []string{"a"}, []model.ManifestName{"b"}, []model.ManifestName{"b"}},
		{"both, with config.parse", true, []string{"a"}, []model.ManifestName{"b"}, []model.ManifestName{"b"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := NewFixture(t, model.NewUserConfigState(tc.args))
			defer f.TearDown()

			setResources := ""
			if len(tc.tiltfileResources) > 0 {
				var rs []string
				for _, mn := range tc.tiltfileResources {
					rs = append(rs, fmt.Sprintf("'%s'", mn))
				}
				setResources = fmt.Sprintf("config.set_enabled_resources([%s])", strings.Join(rs, ", "))
			}

			configParse := ""
			if tc.callConfigParse {
				configParse = `
config.define_string_list('resources', args=True)
config.parse()`
			}

			tiltfile := fmt.Sprintf("%s\n%s\n", setResources, configParse)

			f.File("Tiltfile", tiltfile)

			result, err := f.ExecFile("Tiltfile")
			require.NoError(t, err)

			manifests := []model.Manifest{{Name: "a"}, {Name: "b"}}
			actual, err := MustState(result).EnabledResources(manifests)
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
	args := strings.Split("united states canada mexico panama haiti jamaica peru", " ")

	f := NewFixture(t, model.NewUserConfigState(args))
	defer f.TearDown()

	f.File("Tiltfile", `
config.define_string_list('foo', args=True)
cfg = config.parse()
print(cfg['foo'])
`)

	_, err := f.ExecFile("Tiltfile")
	require.NoError(t, err)

	require.Contains(t, f.PrintOutput(), value.StringSliceToList(args).String())
}

func TestParseKeyword(t *testing.T) {
	foo := strings.Split("republic dominican cuba caribbean greenland el salvador too", " ")
	var args []string
	for _, s := range foo {
		args = append(args, []string{"-foo", s}...)
	}

	f := NewFixture(t, model.NewUserConfigState(args))
	defer f.TearDown()

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
	f := NewFixture(t, model.NewUserConfigState(args))
	defer f.TearDown()

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
	f := NewFixture(t, model.UserConfigState{})
	defer f.TearDown()

	f.File("Tiltfile", `
config.define_string_list('foo', args=True)
config.define_string_list('bar', args=True)
`)

	_, err := f.ExecFile("Tiltfile")
	require.Error(t, err)
	require.Equal(t, "both bar and foo are defined as positional args", err.Error())
}

func TestMultipleArgsSameName(t *testing.T) {
	f := NewFixture(t, model.UserConfigState{})
	defer f.TearDown()

	f.File("Tiltfile", `
config.define_string_list('foo')
config.define_string_list('foo')
`)

	_, err := f.ExecFile("Tiltfile")
	require.Error(t, err)
	require.Equal(t, "foo defined multiple times", err.Error())
}

func TestUndefinedArg(t *testing.T) {
	f := NewFixture(t, model.NewUserConfigState([]string{"-bar", "hello"}))
	defer f.TearDown()

	f.File("Tiltfile", `
config.define_string_list('foo')
config.parse()
`)

	_, err := f.ExecFile("Tiltfile")
	require.Error(t, err)
	require.Equal(t, "flag provided but not defined: -bar", err.Error())
}

func TestUnprovidedArg(t *testing.T) {
	f := NewFixture(t, model.UserConfigState{})
	defer f.TearDown()

	f.File("Tiltfile", `
config.define_string_list('foo')
cfg = config.parse()
print("foo:",cfg['foo'])
`)

	_, err := f.ExecFile("Tiltfile")
	require.Error(t, err)
	require.Contains(t, err.Error(), `key "foo" not in dict`)
}

func TestUnprovidedPositionalArg(t *testing.T) {
	f := NewFixture(t, model.UserConfigState{})
	f.File("Tiltfile", `
config.define_string_list('foo', args=True)
cfg = config.parse()
print("foo:",cfg['foo'])
`)

	_, err := f.ExecFile("Tiltfile")
	require.Error(t, err)
	require.Contains(t, err.Error(), `key "foo" not in dict`)
}

func TestProvidedButUnexpectedPositionalArgs(t *testing.T) {
	f := NewFixture(t, model.NewUserConfigState([]string{"do", "re", "mi"}))
	defer f.TearDown()

	f.File("Tiltfile", `
cfg = config.parse()
`)

	_, err := f.ExecFile("Tiltfile")
	require.Error(t, err)
	require.Equal(t, "positional args were specified, but none were expected (no setting defined with args=True)", err.Error())
}

func TestUsage(t *testing.T) {
	f := NewFixture(t, model.NewUserConfigState([]string{"-bar", "hello"}))
	defer f.TearDown()

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
	f := NewFixture(t, model.NewUserConfigState([]string{"foo", "bar"}))
	defer f.TearDown()

	f.File("Tiltfile", `
config.define_string_list('resources', usage='which resources to load in Tilt', args=True)
config.set_enabled_resources(config.parse()['resources'])
`)

	result, err := f.ExecFile("Tiltfile")
	require.NoError(t, err)

	manifests := []model.Manifest{{Name: "foo"}, {Name: "bar"}, {Name: "baz"}}
	actual, err := MustState(result).EnabledResources(manifests)
	require.NoError(t, err)
	require.Equal(t, manifests[:2], actual)
}

func TestSettingsFromConfigAndArgs(t *testing.T) {
	for _, tc := range []struct {
		name     string
		args     []string
		config   map[string][]string
		expected map[string][]string
	}{
		{
			name:   "args only",
			args:   []string{"-a", "1", "-a", "2", "-b", "3", "-a", "4", "5", "6"},
			config: nil,
			expected: map[string][]string{
				"a": {"1", "2", "4"},
				"b": {"3"},
				"c": {"5", "6"},
			},
		},
		{
			name: "config only",
			args: nil,
			config: map[string][]string{
				"b": {"7", "8"},
				"c": {"9"},
			},
			expected: map[string][]string{
				"b": {"7", "8"},
				"c": {"9"},
			},
		},
		{
			name: "args trump config",
			args: []string{"-a", "1", "-a", "2", "-a", "4", "5", "6"},
			config: map[string][]string{
				"b": {"7", "8"},
				"c": {"9"},
			},
			expected: map[string][]string{
				"a": {"1", "2", "4"},
				"b": {"7", "8"},
				"c": {"5", "6"},
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := NewFixture(t, model.NewUserConfigState(tc.args))
			defer f.TearDown()

			f.File("Tiltfile", `
config.define_string_list('a')
config.define_string_list('b')
config.define_string_list('c', args=True)
cfg = config.parse()
print("a=", cfg.get('a', 'missing'))
print("b=", cfg.get('b', 'missing'))
print("c=", cfg.get('c', 'missing'))
`)
			if tc.config != nil {
				b := &bytes.Buffer{}
				err := json.NewEncoder(b).Encode(tc.config)
				require.NoError(t, err)
				f.File(UserConfigFileName, b.String())
			}

			_, err := f.ExecFile("Tiltfile")
			require.NoError(t, err)

			for _, arg := range []string{"a", "b", "c"} {
				expected := "missing"
				if vs, ok := tc.expected[arg]; ok {
					var s []string
					for _, v := range vs {
						s = append(s, fmt.Sprintf(`"%s"`, v))
					}
					expected = fmt.Sprintf("[%s]", strings.Join(s, ", "))
				}
				require.Contains(t, f.PrintOutput(), fmt.Sprintf("%s= %s", arg, expected))
			}
		})
	}
}

func TestUndefinedArgInConfigFile(t *testing.T) {
	f := NewFixture(t, model.UserConfigState{})
	defer f.TearDown()

	f.File("Tiltfile", `
config.define_string_list('foo')
cfg = config.parse()
print("foo:",cfg.get('foo', []))
`)

	f.File(UserConfigFileName, `{"bar": "1"}`)

	_, err := f.ExecFile("Tiltfile")
	require.Error(t, err)
	require.Contains(t, err.Error(), "specified unknown setting name 'bar'")
}

func TestWrongTypeArgInConfigFile(t *testing.T) {
	f := NewFixture(t, model.UserConfigState{})
	defer f.TearDown()

	f.File("Tiltfile", `
config.define_string_list('foo')
cfg = config.parse()
print("foo:",cfg.get('foo', []))
`)

	f.File(UserConfigFileName, `{"foo": "1"}`)

	_, err := f.ExecFile("Tiltfile")
	require.Error(t, err)
	require.Contains(t, err.Error(), "specified invalid value for setting foo: expected array")
}

func TestConfigParseFromMultipleDirs(t *testing.T) {
	f := NewFixture(t, model.UserConfigState{})
	defer f.TearDown()

	f.File("Tiltfile", `
config.define_string_list('foo')
cfg = config.parse()
include('inc/Tiltfile')
`)

	f.File("inc/Tiltfile", `
cfg = config.parse()
`)

	_, err := f.ExecFile("Tiltfile")
	require.Error(t, err)
	require.Contains(t, err.Error(), "config.parse can only be called from one Tiltfile working directory per run")
	require.Contains(t, err.Error(), f.Path())
	require.Contains(t, err.Error(), f.JoinPath("inc"))
}

func TestDefineSettingAfterParse(t *testing.T) {
	f := NewFixture(t, model.UserConfigState{})
	defer f.TearDown()

	f.File("Tiltfile", `
cfg = config.parse()
config.define_string_list('foo')
`)

	_, err := f.ExecFile("Tiltfile")
	require.Error(t, err)
	require.Contains(t, err.Error(), "config.define_string_list cannot be called after config.parse is called")
}

func TestConfigFileRecordedRead(t *testing.T) {
	f := NewFixture(t, model.UserConfigState{})
	defer f.TearDown()

	f.File("Tiltfile", `
cfg = config.parse()`)

	result, err := f.ExecFile("Tiltfile")
	require.NoError(t, err)

	rs, err := io.GetState(result)
	require.NoError(t, err)
	require.Contains(t, rs.Files, f.JoinPath(UserConfigFileName))
}

func NewFixture(tb testing.TB, userConfigState model.UserConfigState) *starkit.Fixture {
	ret := starkit.NewFixture(tb, NewExtension(userConfigState), io.NewExtension(), include.IncludeFn{})
	ret.UseRealFS()
	return ret
}
