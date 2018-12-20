package docker

import (
	"fmt"
	"testing"

	"github.com/docker/docker/api/types"
	"github.com/stretchr/testify/assert"
)

type buildkitTestCase struct {
	v        types.Version
	expected bool
}

func TestSupportsBuildkit(t *testing.T) {
	cases := []buildkitTestCase{
		{types.Version{APIVersion: "1.37", Experimental: true}, false},
		{types.Version{APIVersion: "1.37", Experimental: false}, false},
		{types.Version{APIVersion: "1.38", Experimental: true}, true},
		{types.Version{APIVersion: "1.38", Experimental: false}, false},
		{types.Version{APIVersion: "1.39", Experimental: true}, true},
		{types.Version{APIVersion: "1.39", Experimental: false}, true},
		{types.Version{APIVersion: "1.40", Experimental: true}, true},
		{types.Version{APIVersion: "1.40", Experimental: false}, true},
		{types.Version{APIVersion: "garbage", Experimental: false}, false},
	}

	for i, c := range cases {
		t.Run(fmt.Sprintf("Case%d", i), func(t *testing.T) {
			assert.Equal(t, c.expected, SupportsBuildkit(c.v))
		})
	}
}

type versionTestCase struct {
	v        types.Version
	expected bool
}

func TestSupported(t *testing.T) {
	cases := []buildkitTestCase{
		{types.Version{APIVersion: "1.22"}, false},
		{types.Version{APIVersion: "1.23"}, true},
		{types.Version{APIVersion: "1.39"}, true},
		{types.Version{APIVersion: "1.40"}, true},
		{types.Version{APIVersion: "garbage"}, false},
	}

	for i, c := range cases {
		t.Run(fmt.Sprintf("Case%d", i), func(t *testing.T) {
			assert.Equal(t, c.expected, SupportedVersion(c.v))
		})
	}
}
