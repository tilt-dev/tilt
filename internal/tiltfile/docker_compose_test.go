package tiltfile

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/windmilleng/tilt/internal/dockercompose"
	"github.com/windmilleng/tilt/internal/testutils/output"
)

var dcYamlManyServices = `version: '3'
services:
  a:
    image: imga
  b:
    image: imgb
  c:
    image: imgc
  d:
    image: imgd
  e:
    image: imge
  f:
    image: imgf`

// ParseConfig must return services topologically sorted wrt dependencies.
func TestParseConfigPreservesServiceOrder(t *testing.T) {
	ctx := output.CtxForTest()
	dcCli := dockercompose.NewFakeDockerComposeClient(t, ctx)
	dcCli.ConfigOutput = dcYamlManyServices

	services, err := parseDCConfig(ctx, dcCli, "doesn't-matter.yml")
	if assert.NoError(t, err) {
		if assert.Len(t, services, 6) {
			for i, name := range []string{"a", "b", "c", "d", "e", "f"} {
				assert.Equal(t, name, services[i].Name)
			}
		}
	}
}
