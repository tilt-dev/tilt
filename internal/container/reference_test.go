package container

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var (
	sel         = MustParseSelector("gcr.io/foo/bar")
	regHostOnly = Registry{Host: "localhost:1234"}
)

func TestNewRefSetWithInvalidRegistryErrors(t *testing.T) {
	reg := Registry{Host: "invalid"}
	assertNewRefSetError(t, sel, reg, "validating registry host")
}

func TestNewRefSetErrorsWithBadLocalRef(t *testing.T) {
	// Force "repository name must not be longer than 255 characters" when assembling LocalRef
	var longname string
	for i := 0; i < 230; i++ {
		longname += "o"
	}
	selector := MustParseSelector(longname)
	reg := Registry{Host: "gcr.io/somewhat/long/hostname"}
	assertNewRefSetError(t, selector, reg, "after applying default registry")
}

func TestNewRefSetErrorsWithBadClusterRef(t *testing.T) {
	// Force "repository name must not be longer than 255 characters" when assembling ClusterRef
	var longname string
	for i := 0; i < 230; i++ {
		longname += "o"
	}
	selector := MustParseSelector(longname)
	reg := Registry{Host: "gcr.io", hostFromCluster: "gcr.io/somewhat/long/hostname"}
	assertNewRefSetError(t, selector, reg, "after applying default registry")
}

func TestNewRefSetEmptyRegistryOK(t *testing.T) {
	_, err := NewRefSet(sel, Registry{})
	assert.NoError(t, err)
}

var cases = []struct {
	name               string
	host               string
	clusterHost        string
	expectedLocalRef   string
	expectedClusterRef string
}{
	{"empty registry", "", "", "gcr.io/foo", "gcr.io/foo"},
	{"host only", "localhost:1234", "", "localhost:1234/gcr.io_foo", "localhost:1234/gcr.io_foo"},
	{"host and clusterHost", "localhost:1234", "registry:1234", "localhost:1234/gcr.io_foo", "registry:1234/gcr.io_foo"},
}

func TestDeriveRefs(t *testing.T) {
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			reg := MustNewRegistryWithHostFromCluster(tc.host, tc.clusterHost)
			refs, err := NewRefSet(MustParseSelector("gcr.io/foo"), reg)
			require.NoError(t, err)

			localRef := refs.LocalRef()
			clusterRef := refs.ClusterRef()

			assert.Equal(t, tc.expectedLocalRef, localRef.String())
			assert.Equal(t, tc.expectedClusterRef, clusterRef.String())
		})
	}
}

func TestWithoutRegistry(t *testing.T) {
	reg := MustNewRegistryWithHostFromCluster("localhost:5000", "registry:5000")
	refs, err := NewRefSet(MustParseSelector("foo"), reg)
	require.NoError(t, err)

	assert.Equal(t, "localhost:5000/foo", FamiliarString(refs.LocalRef()))
	assert.Equal(t, "foo", FamiliarString(refs.WithoutRegistry().LocalRef()))
}

func assertNewRefSetError(t *testing.T, selector RefSelector, reg Registry, expectedErr string) {
	_, err := NewRefSet(selector, reg)
	require.Error(t, err)
	require.Contains(t, err.Error(), expectedErr)
}
