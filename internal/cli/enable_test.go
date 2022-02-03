package cli

import (
	"fmt"
	"strconv"
	"testing"

	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	"github.com/tilt-dev/tilt/internal/controllers/apis/uiresource"
	"github.com/tilt-dev/tilt/internal/testutils/uiresourcebuilder"
	"github.com/tilt-dev/tilt/pkg/apis/core/v1alpha1"
	"github.com/tilt-dev/tilt/pkg/model"
)

func TestEnable(t *testing.T) {
	for _, tc := range []struct {
		name            string
		args            []string
		expectedEnabled []string
		expectedError   string
	}{
		{
			"normal",
			[]string{"disabled_b", "disabled_c"},
			[]string{"enabled_a", "enabled_b", "enabled_c", "disabled_b", "disabled_c", "(Tiltfile)"},
			"",
		},
		{
			"only",
			[]string{"--only", "disabled_b"},
			[]string{"disabled_b", "(Tiltfile)"},
			"",
		},
		{
			"all",
			[]string{"--all"},
			[]string{"enabled_a", "enabled_b", "enabled_c", "disabled_a", "disabled_b", "disabled_c", "(Tiltfile)"},
			"",
		},
		{
			"all+names",
			[]string{"--all", "enabled_b"},
			nil,
			"cannot use --all with resource names",
		},
		{
			"no names",
			nil,
			nil,
			"must specify at least one resource",
		},
		{
			"nonexistent resource",
			[]string{"foo"},
			nil,
			"no such resource \"foo\"",
		},
		{
			"Tiltfile",
			[]string{"(Tiltfile)"},
			nil,
			"(Tiltfile) cannot be enabled or disabled",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := newEnableFixture(t)
			defer f.TearDown()

			f.createResources()

			cmd := enableCmd{}
			c := cmd.register()
			err := c.Flags().Parse(tc.args)
			require.NoError(t, err)
			err = cmd.run(f.ctx, c.Flags().Args())
			if tc.expectedError != "" {
				require.Error(t, err)
				require.Contains(t, err.Error(), tc.expectedError)
				// if there's an error, the set of enabled resources shouldn't change
				tc.expectedEnabled = []string{"enabled_a", "enabled_b", "enabled_c", "(Tiltfile)"}
			} else {
				require.NoError(t, err)
			}

			require.ElementsMatch(t, tc.expectedEnabled, f.enabledResources())
		})
	}
}

type enableFixture struct {
	*serverFixture
}

func newEnableFixture(t *testing.T) enableFixture {
	return enableFixture{newServerFixture(t)}
}

// makes 7 resources: enabled_a, enabled_b, enabled_c, disabled_a, disabled_b, disabled_c, (Tiltfile)
// The first six are initially enabled/disabled according to their names
// (Tiltfile) is always enabled
func (f enableFixture) createResources() {
	for _, isDisabled := range []bool{true, false} {
		for _, n := range []string{"a", "b", "c"} {
			name := fmt.Sprintf("enabled_%s", n)
			if isDisabled {
				name = fmt.Sprintf("disabled_%s", n)
			}

			source := v1alpha1.DisableSource{
				ConfigMap: &v1alpha1.ConfigMapDisableSource{
					Name: fmt.Sprintf("disable-%s", name),
					Key:  "isDisabled",
				},
			}

			uir := uiresourcebuilder.New(name).WithDisableSource(source).Build()
			err := f.client.Create(f.ctx, uir)
			require.NoError(f.T(), err)

			cm := v1alpha1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{Name: source.ConfigMap.Name},
				Data:       map[string]string{source.ConfigMap.Key: strconv.FormatBool(isDisabled)},
			}
			err = f.client.Create(f.ctx, &cm)
			require.NoError(f.T(), err)
		}
	}

	uir := uiresourcebuilder.New(string(model.MainTiltfileManifestName)).Build()
	err := f.client.Create(f.ctx, uir)
	require.NoError(f.T(), err)
}

func (f enableFixture) enabledResources() []string {
	var result []string

	var uirs v1alpha1.UIResourceList
	err := f.client.List(f.ctx, &uirs)
	require.NoError(f.T(), err)
	for _, uir := range uirs.Items {
		drs, err := uiresource.DisableResourceStatus(func(name string) (v1alpha1.ConfigMap, error) {
			var cm v1alpha1.ConfigMap
			err := f.client.Get(f.ctx, types.NamespacedName{Name: name}, &cm)
			return cm, err
		}, uir.Status.DisableStatus.Sources)
		require.NoError(f.T(), err)

		if drs.State == v1alpha1.DisableStateEnabled {
			result = append(result, uir.Name)
		}
	}

	return result
}
