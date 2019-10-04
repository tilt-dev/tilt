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
	actual := provideWebVersion(model.TiltBuild{Version: "0.10.13", Date: "", Dev: false})

	assert.Equal(t, expected, string(actual))
}
