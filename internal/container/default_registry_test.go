package container

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

var namedTaggedTestCases = []struct {
	defaultRegistry Registry
	name            string
	expected        string
}{
	{"myreg.com", "gcr.io/foo/bar:deadbeef", "myreg.com/gcr.io_foo_bar"},
	{"aws_account_id.dkr.ecr.region.amazonaws.com/bar", "gcr.io/baz/foo/bar:deadbeef", "aws_account_id.dkr.ecr.region.amazonaws.com/bar/gcr.io_baz_foo_bar"},
}

var namedTestCases = []struct {
	defaultRegistry Registry
	name            string
	expected        string
}{
	{"myreg.com", "gcr.io/foo/bar", "myreg.com/gcr.io_foo_bar"},
	{"myreg.com", "gcr.io/foo/bar", "myreg.com/gcr.io_foo_bar"},
	{"aws_account_id.dkr.ecr.region.amazonaws.com/bar", "gcr.io/baz/foo/bar", "aws_account_id.dkr.ecr.region.amazonaws.com/bar/gcr.io_baz_foo_bar"},
	{"gcr.io/foo", "docker.io/library/busybox", "gcr.io/foo/busybox"},
	{"gcr.io/foo", "bar", "gcr.io/foo/bar"},
}

func TestReplaceTaggedRefDomain(t *testing.T) {
	for i, tc := range namedTaggedTestCases {
		t.Run(fmt.Sprintf("Test Case #%d", i), func(t *testing.T) {
			rs := NewRefSelector(MustParseNamedTagged(tc.name))
			actual, err := ReplaceRegistry(tc.defaultRegistry, rs)
			assert.NoError(t, err)
			assert.Equal(t, tc.expected, actual.String())
		})
	}
}

func TestReplaceNamed(t *testing.T) {
	for i, tc := range namedTestCases {
		t.Run(fmt.Sprintf("Test case #%d", i), func(t *testing.T) {
			rs := NewRefSelector(MustParseNamed(tc.name))
			actual, err := ReplaceRegistry(tc.defaultRegistry, rs)
			assert.NoError(t, err)
			assert.Equal(t, tc.expected, actual.String())
		})
	}
}
