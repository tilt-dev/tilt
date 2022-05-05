package v1alpha1_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tilt-dev/tilt/pkg/apis/core/v1alpha1"
)

func TestRegistryHosting_Validate_HostError(t *testing.T) {
	var newRegistryError = []struct {
		defaultReg    string
		expectedError string
	}{
		{"", "host: Required value"},
		{"invalid", `host: Invalid value: "invalid": repository name must be canonical`},
		{"foo/bar/baz", `host: Invalid value: "foo/bar/baz": repository name must be canonical`},
	}

	for _, tc := range newRegistryError {
		name := tc.defaultReg
		if name == "" {
			name = "(empty)"
		}
		t.Run(name, func(t *testing.T) {
			reg := &v1alpha1.RegistryHosting{Host: tc.defaultReg}
			errs := reg.Validate(context.Background())
			if assert.Len(t, errs, 1) {
				require.EqualError(t, errs[0], tc.expectedError)
			}
		})
	}
}

func TestRegistryHosting_Validate_HostFromClusterError(t *testing.T) {
	var newRegistryWithHostFromClusterError = []struct {
		host          string
		clusterHost   string
		expectedError string
	}{
		{"invalid", "grc.io/valid", `host: Invalid value: "invalid": repository name must be canonical`},
		{"grc.io/valid", "invalid", `hostFromContainerRuntime: Invalid value: "invalid": repository name must be canonical`},
		{"", "grc.io/valid", "host: Required value"},
	}

	for i, tc := range newRegistryWithHostFromClusterError {
		t.Run(fmt.Sprintf("Test case #%d", i), func(t *testing.T) {
			reg := &v1alpha1.RegistryHosting{
				Host:                     tc.host,
				HostFromContainerRuntime: tc.clusterHost,
			}
			errs := reg.Validate(context.Background())
			if assert.Len(t, errs, 1) {
				require.EqualError(t, errs[0], tc.expectedError)
			}
		})
	}
}
