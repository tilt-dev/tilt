package cli

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestProvideWebVersion(t *testing.T) {
	assert.Equal(t, fmt.Sprintf("v%s", devVersion), string(provideWebVersion(provideBuildInfo())))
}

func TestBuildVersion(t *testing.T) {
	bi := BuildInfo{
		Version: "0.7.1",
		Date:    "2018-04-01",
		Dev:     false,
	}

	expected := "0.7.1"
	actual := bi.FullVersion()
	assert.Equal(t, expected, actual)
}

func TestDevBuildVersion(t *testing.T) {
	bi := BuildInfo{
		Version: "0.7.1",
		Date:    "2018-04-01",
		Dev:     true,
	}

	expected := "0.7.1-dev"
	actual := bi.FullVersion()
	assert.Equal(t, expected, actual)
}
