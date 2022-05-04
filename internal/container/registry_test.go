package container

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tilt-dev/tilt/pkg/apis/core/v1alpha1"
)

func TestReplaceTaggedRefDomain(t *testing.T) {
	var namedTaggedTestCases = []struct {
		defaultRegistry string
		name            string
		expected        string
	}{
		{"myreg.com", "gcr.io/foo/bar:deadbeef", "myreg.com/gcr.io_foo_bar"},
		{"aws_account_id.dkr.ecr.region.amazonaws.com/bar", "gcr.io/baz/foo/bar:deadbeef", "aws_account_id.dkr.ecr.region.amazonaws.com/bar/gcr.io_baz_foo_bar"},
	}

	for i, tc := range namedTaggedTestCases {
		t.Run(fmt.Sprintf("Test Case #%d", i), func(t *testing.T) {
			reg := &v1alpha1.RegistryHosting{Host: tc.defaultRegistry}
			assertReplaceRegistryForLocal(t, reg, tc.name, tc.expected)
		})
	}
}

func TestReplaceNamed(t *testing.T) {
	var namedTestCases = []struct {
		defaultRegistry string
		name            string
		expected        string
	}{
		{"myreg.com", "gcr.io/foo/bar", "myreg.com/gcr.io_foo_bar"},
		{"myreg.com", "gcr.io/foo/bar", "myreg.com/gcr.io_foo_bar"},
		{"aws_account_id.dkr.ecr.region.amazonaws.com/bar", "gcr.io/baz/foo/bar", "aws_account_id.dkr.ecr.region.amazonaws.com/bar/gcr.io_baz_foo_bar"},
		{"gcr.io/foo", "docker.io/library/busybox", "gcr.io/foo/busybox"},
		{"gcr.io/foo", "bar", "gcr.io/foo/bar"},
		{"myreg.com", "myreg.com/bar", "myreg.com/bar"},
		{"myreg.com:5000", "myreg.com:5000/bar", "myreg.com:5000/bar"},
		{"myreg.com:5000", "myreg.com/bar", "myreg.com:5000/myreg.com_bar"},
		{"myreg.com", "myreg.com:5000/bar", "myreg.com/myreg.com_5000_bar"},
	}

	for i, tc := range namedTestCases {
		t.Run(fmt.Sprintf("Test case #%d", i), func(t *testing.T) {
			reg := &v1alpha1.RegistryHosting{Host: tc.defaultRegistry}
			assertReplaceRegistryForLocal(t, reg, tc.name, tc.expected)
		})
	}
}

func TestReplaceRefForCluster(t *testing.T) {
	var refForClusterCases = []struct {
		host            string
		clusterHost     string
		name            string
		expectedLocal   string
		expectedCluster string
	}{
		{"myreg.com", "", "gcr.io/foo/bar", "myreg.com/gcr.io_foo_bar", "myreg.com/gcr.io_foo_bar"},
		{"myreg.com", "myreg.com", "gcr.io/foo/bar", "myreg.com/gcr.io_foo_bar", "myreg.com/gcr.io_foo_bar"},
		{"localhost:1234", "registry:1234", "gcr.io/foo/bar", "localhost:1234/gcr.io_foo_bar", "registry:1234/gcr.io_foo_bar"},
	}

	for i, tc := range refForClusterCases {
		t.Run(fmt.Sprintf("Test case #%d", i), func(t *testing.T) {
			reg := &v1alpha1.RegistryHosting{
				Host:                     tc.host,
				HostFromContainerRuntime: tc.clusterHost,
			}
			assertReplaceRegistryForLocal(t, reg, tc.name, tc.expectedLocal)
			assertReplaceRegistryForCluster(t, reg, tc.name, tc.expectedCluster)
		})
	}
}

func TestRegistryFromCluster(t *testing.T) {
	registryHosting := func(host string) *v1alpha1.RegistryHosting {
		return &v1alpha1.RegistryHosting{
			Host:                     host,
			HostFromClusterNetwork:   "localhost:12345/cluster-network",
			HostFromContainerRuntime: "localhost:12345/container-runtime",
			Help:                     "fake-help",
		}
	}

	tests := []struct {
		name               string
		cluster            v1alpha1.Cluster
		expectedHost       string
		expectedSingleName string
		expectedLocal      bool
	}{
		{name: "Empty"},
		{
			name: "DefaultNoLocal",
			cluster: v1alpha1.Cluster{
				Spec: v1alpha1.ClusterSpec{
					DefaultRegistry: &v1alpha1.RegistryHosting{
						Host:       "registry.example.com",
						SingleName: "fake-repo",
					},
				},
			},
			expectedHost:       "registry.example.com",
			expectedSingleName: "fake-repo",
		},
		{
			name: "DefaultWithLocal",
			cluster: v1alpha1.Cluster{
				Spec: v1alpha1.ClusterSpec{
					DefaultRegistry: &v1alpha1.RegistryHosting{
						Host:       "registry.example.com",
						SingleName: "fake-repo",
					},
				},
				Status: v1alpha1.ClusterStatus{
					Registry: registryHosting("localhost:12345"),
				},
			},
			expectedHost:  "localhost:12345",
			expectedLocal: true,
		},
		{
			name: "LocalNoDefault",
			cluster: v1alpha1.Cluster{
				Status: v1alpha1.ClusterStatus{
					Registry: registryHosting("localhost:7890"),
				},
			},
			expectedHost:  "localhost:7890",
			expectedLocal: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reg, err := RegistryFromCluster(&tt.cluster)
			require.NoError(t, err, "Registry error")
			if tt.expectedHost == "" {
				return
			}

			require.NotNil(t, reg)
			require.Equal(t, tt.expectedHost, reg.Host, "Registry host")
			require.Equal(t, tt.expectedSingleName, reg.SingleName, "Registry single name")
			if tt.expectedLocal {
				require.Equal(t, "localhost:12345/container-runtime", reg.HostFromContainerRuntime,
					"Registry host from container runtime")
			}
		})
	}
}

func TestRegistryFromCluster_Error(t *testing.T) {
	cluster := v1alpha1.Cluster{
		Status: v1alpha1.ClusterStatus{Error: "fake cluster error"},
	}
	reg, err := RegistryFromCluster(&cluster)
	require.EqualError(t, err, "cluster not ready: fake cluster error")
	require.Nil(t, reg)
}

func assertReplaceRegistryForLocal(t *testing.T, reg *v1alpha1.RegistryHosting, orig string, expected string) {
	rs := NewRefSelector(MustParseNamed(orig))
	actual, err := ReplaceRegistryForLocalRef(rs, reg)
	require.NoError(t, err)
	assert.Equal(t, expected, actual.String())
}

func assertReplaceRegistryForCluster(t *testing.T, reg *v1alpha1.RegistryHosting, orig string, expected string) {
	rs := NewRefSelector(MustParseNamed(orig))
	actual, err := ReplaceRegistryForContainerRuntimeRef(rs, reg)
	require.NoError(t, err)
	assert.Equal(t, expected, actual.String())
}
