package uibutton

import (
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/tilt-dev/tilt/pkg/apis/core/v1alpha1"
)

func CancelButtonName(resourceName string) string {
	return fmt.Sprintf("%s-cancel", resourceName)
}

func CancelButton(resourceName string) *v1alpha1.UIButton {
	return &v1alpha1.UIButton{
		ObjectMeta: metav1.ObjectMeta{
			Name: CancelButtonName(resourceName),
			Annotations: map[string]string{
				v1alpha1.AnnotationButtonType: v1alpha1.ButtonTypeCancelUpdate,
			},
		},
		Spec: v1alpha1.UIButtonSpec{
			Location: v1alpha1.UIComponentLocation{
				ComponentID:   resourceName,
				ComponentType: v1alpha1.ComponentTypeResource,
			},
			Text:     "Cancel",
			IconName: "cancel",
		},
	}
}
