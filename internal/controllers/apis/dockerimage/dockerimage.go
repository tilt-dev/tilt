package dockerimage

import (
	"fmt"

	"github.com/tilt-dev/tilt/pkg/apis"
	"github.com/tilt-dev/tilt/pkg/model"
)

// Generate the name for the DockerImage API object from an ImageTarget and ManifestName.
func GetName(mn model.ManifestName, id model.TargetID) string {
	return apis.SanitizeName(fmt.Sprintf("%s:%s", mn.String(), id.Name))
}
