package cli

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/windmilleng/tilt/pkg/model"
)

func TestProvideDevWebVersion(t *testing.T) {
	assert.Equal(t, devVersion, string(provideWebVersion(provideTiltInfo())))
}

func TestProvideProdWebVersion(t *testing.T) {
	expected := fmt.Sprintf("v%s", devVersion)
	actual := provideWebVersion(model.TiltBuild{"0.10.13", "", false})

	assert.Equal(t, expected, string(actual))
}
