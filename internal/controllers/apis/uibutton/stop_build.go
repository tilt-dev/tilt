package uibutton

import (
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/tilt-dev/tilt/pkg/apis/core/v1alpha1"
)

func StopBuildButtonName(resourceName string) string {
	return fmt.Sprintf("%s-stopbuild", resourceName)
}

func StopBuildButton(resourceName string) *v1alpha1.UIButton {
	return &v1alpha1.UIButton{
		ObjectMeta: metav1.ObjectMeta{
			Name: StopBuildButtonName(resourceName),
			Annotations: map[string]string{
				v1alpha1.AnnotationButtonType: v1alpha1.ButtonTypeStopBuild,
			},
		},
		Spec: v1alpha1.UIButtonSpec{
			Location: v1alpha1.UIComponentLocation{
				ComponentID:   resourceName,
				ComponentType: v1alpha1.ComponentTypeResource,
			},
			Text:     "Stop Build",
			IconName: "cancel",
		},
	}
}
