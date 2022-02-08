package cli

import (
	"bytes"
	"fmt"
	"os"
	"runtime"
	"strings"
	"testing"

	"github.com/alessio/shellescape"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	"github.com/tilt-dev/tilt/pkg/apis/core/v1alpha1"
	"github.com/tilt-dev/tilt/pkg/model"
	"github.com/tilt-dev/wmclient/pkg/analytics"
)

func TestArgsClear(t *testing.T) {
	f := newServerFixture(t)
	defer f.TearDown()

	createTiltfile(f, []string{"foo", "bar"})

	cmd := newArgsCmd()
	c := cmd.register()
	err := c.Flags().Parse([]string{"--clear"})
	require.NoError(t, err)
	err = cmd.run(f.ctx, c.Flags().Args())
	require.NoError(t, err)

	require.Equal(t, 0, len(getTiltfile(f).Spec.Args))
	require.Equal(t, []analytics.CountEvent{
		{Name: "cmd.args", Tags: map[string]string{"clear": "true"}, N: 1},
	}, f.analytics.Counts)
}

func TestArgsNewValue(t *testing.T) {
	f := newServerFixture(t)
	defer f.TearDown()

	createTiltfile(f, []string{"foo", "bar"})

	cmd := newArgsCmd()
	c := cmd.register()
	err := c.Flags().Parse([]string{"--", "--foo", "bar"})
	require.NoError(t, err)
	err = cmd.run(f.ctx, c.Flags().Args())
	require.NoError(t, err)

	require.Equal(t, []string{"--foo", "bar"}, getTiltfile(f).Spec.Args)
	require.Equal(t, []analytics.CountEvent{
		{Name: "cmd.args", Tags: map[string]string{"set": "true"}, N: 1},
	}, f.analytics.Counts)

}

func TestArgsClearAndNewValue(t *testing.T) {
	f := newServerFixture(t)
	defer f.TearDown()

	createTiltfile(f, []string{"foo", "bar"})

	cmd := newArgsCmd()
	c := cmd.register()
	err := c.Flags().Parse([]string{"--clear", "--", "--foo", "bar"})
	require.NoError(t, err)
	err = cmd.run(f.ctx, c.Flags().Args())
	require.Error(t, err)
	require.Contains(t, err.Error(), "--clear cannot be specified with other values")
}

func TestArgsNoChange(t *testing.T) {
	f := newServerFixture(t)
	defer f.TearDown()

	createTiltfile(f, []string{"foo", "bar"})

	cmd := newArgsCmd()
	out := &bytes.Buffer{}
	cmd.streams.Out = out
	cmd.streams.ErrOut = out
	c := cmd.register()
	err := c.Flags().Parse([]string{"foo", "bar"})
	require.NoError(t, err)
	err = cmd.run(f.ctx, c.Flags().Args())
	require.NoError(t, err)
	require.Contains(t, out.String(), "no action taken")
}

func TestArgsEdit(t *testing.T) {
	editorForString := func(contents string) string {
		switch runtime.GOOS {
		case "windows":
			// This is trying to minimize windows weirdness:
			// 1. If EDITOR includes a ` ` and a `\`, then the editor library will prepend a cmd /c,
			//    but then pass the whole $EDITOR as a single element of argv, while cmd /c
			//    seems to want everything as separate argvs. Since we're on Windows, any paths
			//    we get will have a `\`.
			// 2. Windows' echo gave surprising quoting behavior that I didn't take the time to understand.
			// So: generate one txt file that contains the desired contents and one bat file that
			// simply writes the txt file to the first arg, so that the EDITOR we pass to the editor library
			// has no spaces or quotes.
			argFile, err := os.CreateTemp(t.TempDir(), "newargs*.txt")
			require.NoError(t, err)
			_, err = argFile.WriteString(contents)
			require.NoError(t, err)
			require.NoError(t, argFile.Close())
			f, err := os.CreateTemp(t.TempDir(), "writeargs*.bat")
			require.NoError(t, err)
			_, err = f.WriteString(fmt.Sprintf(`type %s > %%1`, argFile.Name()))
			require.NoError(t, err)
			err = f.Close()
			require.NoError(t, err)
			return f.Name()
		default:
			return fmt.Sprintf("echo %s >", shellescape.Quote(contents))
		}
	}

	for _, tc := range []struct {
		name          string
		contents      string
		expectedArgs  []string
		expectedError string
	}{
		{"simple", "baz quu", []string{"baz", "quu"}, ""},
		{"quotes", "baz 'quu quz'", []string{"baz", "quu quz"}, ""},
		{"comments ignored", " # test comment\n1 2\n  # second test comment", []string{"1", "2"}, ""},
		{"parse error", "baz 'quu", nil, "Unterminated single-quoted string"},
		{"only comments", "# these are the tilt args", nil, "must have exactly one non-comment line, found zero. If you want to clear the args, use `tilt args --clear`"},
		{"multiple lines", "foo\nbar\n", nil, "cannot have multiple non-comment lines"},
		{"empty lines ignored", "1 2\n\n\n", []string{"1", "2"}, ""},
		{"dashes", "--foo --bar", []string{"--foo", "--bar"}, ""},
		{"quoted hash", "1 '2 # not a comment'", []string{"1", "2 # not a comment"}, ""},
		// TODO - fix comment parsing so the below passes
		// {"mid-line comment", "1 2 # comment", []string{"1", "2"}, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := newServerFixture(t)
			defer f.TearDown()

			origEditor := os.Getenv("EDITOR")
			contents := tc.contents
			if runtime.GOOS == "windows" {
				contents = strings.ReplaceAll(contents, "\n", "\r\n")
			}
			err := os.Setenv("EDITOR", editorForString(contents))
			require.NoError(t, err)
			defer func() {
				err := os.Setenv("EDITOR", origEditor)
				require.NoError(t, err)
			}()

			originalArgs := []string{"foo", "bar"}
			createTiltfile(f, originalArgs)

			cmd := newArgsCmd()
			c := cmd.register()
			err = c.Flags().Parse(nil)
			require.NoError(t, err)
			err = cmd.run(f.ctx, c.Flags().Args())
			if tc.expectedError != "" {
				require.Error(t, err)
				require.Contains(t, err.Error(), tc.expectedError)
			} else {
				require.NoError(t, err)
			}

			expectedArgs := originalArgs
			if tc.expectedArgs != nil {
				expectedArgs = tc.expectedArgs
			}
			require.Equal(t, expectedArgs, getTiltfile(f).Spec.Args)
			var expectedCounts []analytics.CountEvent
			if tc.expectedError == "" {
				expectedCounts = []analytics.CountEvent{
					{Name: "cmd.args", Tags: map[string]string{"edit": "true"}, N: 1},
				}
			}
			require.Equal(t, expectedCounts, f.analytics.Counts)
		})
	}
}

func createTiltfile(f *serverFixture, args []string) {
	tf := v1alpha1.Tiltfile{
		ObjectMeta: metav1.ObjectMeta{
			Name: model.MainTiltfileManifestName.String(),
		},
		Spec:   v1alpha1.TiltfileSpec{Args: args},
		Status: v1alpha1.TiltfileStatus{},
	}
	err := f.client.Create(f.ctx, &tf)
	require.NoError(f.T(), err)
}

func getTiltfile(f *serverFixture) *v1alpha1.Tiltfile {
	var tf v1alpha1.Tiltfile
	err := f.client.Get(f.ctx, types.NamespacedName{Name: model.MainTiltfileManifestName.String()}, &tf)
	require.NoError(f.T(), err)
	return &tf
}
