package tiltfile

import (
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/tilt-dev/tilt/pkg/apis"
	"github.com/tilt-dev/tilt/pkg/apis/core/v1alpha1"
	"github.com/tilt-dev/tilt/pkg/model"
)

func MainTiltfile(filename string, args []string) *v1alpha1.Tiltfile {
	name := model.MainTiltfileManifestName.String()
	fwName := apis.SanitizeName(fmt.Sprintf("%s:%s", model.TargetTypeConfigs, name))
	return &v1alpha1.Tiltfile{
		ObjectMeta: metav1.ObjectMeta{Name: name},
		Spec: v1alpha1.TiltfileSpec{
			Path: filename,
			Args: args,
			RestartOn: &v1alpha1.RestartOnSpec{
				FileWatches: []string{fwName},
			},
		},
	}
}
