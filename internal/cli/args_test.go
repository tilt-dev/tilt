package cli

import (
	"fmt"
	"os"
	"testing"

	"github.com/alessio/shellescape"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	"github.com/tilt-dev/tilt/pkg/apis/core/v1alpha1"
	"github.com/tilt-dev/tilt/pkg/model"
)

func TestArgsClear(t *testing.T) {
	f := newServerFixture(t)
	defer f.TearDown()

	createTiltfile(f, []string{"foo", "bar"})

	cmd := argsCmd{}
	c := cmd.register()
	err := c.Flags().Parse([]string{"--clear"})
	require.NoError(t, err)
	err = cmd.run(f.ctx, c.Flags().Args())
	require.NoError(t, err)

	require.Equal(t, 0, len(getTiltfile(f).Spec.Args))
}

func TestArgsNewValue(t *testing.T) {
	f := newServerFixture(t)
	defer f.TearDown()

	createTiltfile(f, []string{"foo", "bar"})

	cmd := argsCmd{}
	c := cmd.register()
	err := c.Flags().Parse([]string{"--", "--foo", "bar"})
	require.NoError(t, err)
	err = cmd.run(f.ctx, c.Flags().Args())
	require.NoError(t, err)

	require.Equal(t, []string{"--foo", "bar"}, getTiltfile(f).Spec.Args)
}

func TestArgsClearAndNewValue(t *testing.T) {
	f := newServerFixture(t)
	defer f.TearDown()

	createTiltfile(f, []string{"foo", "bar"})

	cmd := argsCmd{}
	c := cmd.register()
	err := c.Flags().Parse([]string{"--clear", "--", "--foo", "bar"})
	require.NoError(t, err)
	err = cmd.run(f.ctx, c.Flags().Args())
	require.Error(t, err)
	require.Contains(t, err.Error(), "--clear cannot be specified with other values")
}

func TestArgsEdit(t *testing.T) {
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
			err := os.Setenv("EDITOR", fmt.Sprintf("echo %s >", shellescape.Quote(tc.contents)))
			require.NoError(t, err)
			defer func() {
				err := os.Setenv("EDITOR", origEditor)
				require.NoError(t, err)
			}()

			originalArgs := []string{"foo", "bar"}
			createTiltfile(f, originalArgs)

			cmd := argsCmd{}
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
