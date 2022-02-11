package tiltfile

import (
	"fmt"
	"path/filepath"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/tilt-dev/tilt/internal/controllers/apis/uibutton"
	"github.com/tilt-dev/tilt/internal/ospath"
	"github.com/tilt-dev/tilt/pkg/apis"
	"github.com/tilt-dev/tilt/pkg/apis/core/v1alpha1"
	"github.com/tilt-dev/tilt/pkg/model"
)

// Resolve a filename to its "best" version.
// On any error, just return the original filename.
func ResolveFilename(filename string) string {
	resolved, err := ospath.RealAbs(filename)
	if err == nil {
		return resolved
	}

	resolved, err = filepath.Abs(filename)
	if err == nil {
		return resolved
	}

	return filename
}

func MainTiltfile(filename string, args []string) *v1alpha1.Tiltfile {
	name := model.MainTiltfileManifestName.String()
	fwName := apis.SanitizeName(fmt.Sprintf("%s:%s", model.TargetTypeConfigs, name))
	return &v1alpha1.Tiltfile{
		ObjectMeta: metav1.ObjectMeta{Name: name},
		Spec: v1alpha1.TiltfileSpec{
			Path: ResolveFilename(filename),
			Args: args,
			RestartOn: &v1alpha1.RestartOnSpec{
				FileWatches: []string{fwName},
			},
			StopOn: &v1alpha1.StopOnSpec{
				UIButtons: []string{uibutton.CancelButtonName(name)},
			},
		},
	}
}
