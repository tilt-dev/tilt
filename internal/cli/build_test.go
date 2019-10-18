package cli

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/windmilleng/tilt/pkg/model"
)

func TestProvideDevWebVersion(t *testing.T) {
	assert.Equal(t, fmt.Sprintf("v%s", devVersion), string(provideWebVersion(provideTiltInfo())))
}

func TestProvideProdWebVersion(t *testing.T) {
	expected := fmt.Sprintf("v%s", devVersion)
	actual := provideWebVersion(model.TiltBuild{Version: devVersion, Date: "", Dev: false})

	assert.Equal(t, expected, string(actual))
}
