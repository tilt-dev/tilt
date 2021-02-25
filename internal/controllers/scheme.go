package controllers

import (
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"

	corev1alpha1 "github.com/tilt-dev/tilt/pkg/apis/core/v1alpha1"
)

func NewScheme() *runtime.Scheme {
	scheme := runtime.NewScheme()

	// modify pkg/apis/core/v1alpha1/register.go to get new types within core group registered
	utilruntime.Must(corev1alpha1.AddToScheme(scheme))

	return scheme
}
