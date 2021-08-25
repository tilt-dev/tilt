package v1alpha1

import (
	"k8s.io/apimachinery/pkg/runtime"

	"github.com/tilt-dev/tilt/tools/devlog"
)

func addDefaultingFuncs(scheme *runtime.Scheme) error {
	return RegisterDefaults(scheme)
}

func SetDefaults_UIButton(obj *UIButton) {
	devlog.Logf("setting defaults for button %s. It has %d inputs.", obj.Name, len(obj.Spec.Inputs))
	for _, input := range obj.Spec.Inputs {
		if input.Bool != nil {
			if input.Bool.TrueString == nil {
				v := "true"
				input.Bool.TrueString = &v
			}
			if input.Bool.FalseString == nil {
				v := "false"
				input.Bool.FalseString = &v
			}
		}
	}
}
