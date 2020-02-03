package container

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var namedTaggedTestCases = []struct {
	defaultRegistry string
	name            string
	expected        string
}{
	{"myreg.com", "gcr.io/foo/bar:deadbeef", "myreg.com/gcr.io_foo_bar"},
	{"aws_account_id.dkr.ecr.region.amazonaws.com/bar", "gcr.io/baz/foo/bar:deadbeef", "aws_account_id.dkr.ecr.region.amazonaws.com/bar/gcr.io_baz_foo_bar"},
}

func TestReplaceTaggedRefDomain(t *testing.T) {
	for i, tc := range namedTaggedTestCases {
		t.Run(fmt.Sprintf("Test Case #%d", i), func(t *testing.T) {
			reg := MustNewRegistry(tc.defaultRegistry)
			assertReplaceRegistryForLocal(t, reg, tc.name, tc.expected)
		})
	}
}

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
}

func TestReplaceNamed(t *testing.T) {
	for i, tc := range namedTestCases {
		t.Run(fmt.Sprintf("Test case #%d", i), func(t *testing.T) {
			reg := MustNewRegistry(tc.defaultRegistry)
			assertReplaceRegistryForLocal(t, reg, tc.name, tc.expected)
		})
	}
}

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

func TestReplaceRefForCluster(t *testing.T) {
	for i, tc := range refForClusterCases {
		t.Run(fmt.Sprintf("Test case #%d", i), func(t *testing.T) {
			reg := MustNewRegistryWithHostFromCluster(tc.host, tc.clusterHost)
			assertReplaceRegistryForLocal(t, reg, tc.name, tc.expectedLocal)
			assertReplaceRegistryForCluster(t, reg, tc.name, tc.expectedCluster)
		})
	}
}

var newRegistryError = []struct {
	defaultReg    string
	expectedError string
}{
	{"invalid", "repository name must be canonical"},
	{"foo/bar/baz", "repository name must be canonical"},
}

func TestNewRegistryError(t *testing.T) {
	for i, tc := range newRegistryError {
		t.Run(fmt.Sprintf("Test case #%d", i), func(t *testing.T) {
			_, err := NewRegistry(tc.defaultReg)
			require.Error(t, err)
			require.Contains(t, err.Error(), tc.expectedError)
		})
	}
}

var newRegistryWithHostFromClusterError = []struct {
	host          string
	clusterHost   string
	expectedError string
}{
	{"invalid", "grc.io/valid", "repository name must be canonical"},
	{"grc.io/valid", "invalid", "repository name must be canonical"},
	{"", "grc.io/valid", "without providing Host"},
}

func TestNewRegistryWithHostFromClusterError(t *testing.T) {
	for i, tc := range newRegistryWithHostFromClusterError {
		t.Run(fmt.Sprintf("Test case #%d", i), func(t *testing.T) {
			_, err := NewRegistryWithHostFromCluster(tc.host, tc.clusterHost)
			require.Error(t, err)
			require.Contains(t, err.Error(), tc.expectedError)
		})
	}
}

func TestNewRegistryEmptyOK(t *testing.T) {
	_, err := NewRegistryWithHostFromCluster("", "")
	require.NoError(t, err)
}

func assertReplaceRegistryForLocal(t *testing.T, reg Registry, orig string, expected string) {
	rs := NewRefSelector(MustParseNamed(orig))
	actual, err := reg.ReplaceRegistryForLocalRef(rs)
	require.NoError(t, err)
	assert.Equal(t, expected, actual.String())
}

func assertReplaceRegistryForCluster(t *testing.T, reg Registry, orig string, expected string) {
	rs := NewRefSelector(MustParseNamed(orig))
	actual, err := reg.ReplaceRegistryForClusterRef(rs)
	require.NoError(t, err)
	assert.Equal(t, expected, actual.String())
}
