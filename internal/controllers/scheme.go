package controllers

import (
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"

	corev1alpha1 "github.com/tilt-dev/tilt/pkg/apis/core/v1alpha1"
)

// Scheme is a type alias for runtime.Scheme for DI convenience.
//
// It's unfortunately not possible to share the scheme with the API server because it registers
// types slightly differently based on how it handles the canonical version of each resource,
// so weird errors will result.
//
// As a result, the controller manager maintains its own (largely duplicative) scheme, and the
// type alias ensures that there's no ambiguity in dependency injection.
type Scheme runtime.Scheme

// RuntimeScheme is the actual scheme object for interop with controller-runtime types.
func (s *Scheme) RuntimeScheme() *runtime.Scheme {
	return (*runtime.Scheme)(s)
}

// NewScheme creates the scheme for the controller manager types.
func NewScheme() *Scheme {
	scheme := runtime.NewScheme()

	// modify pkg/apis/core/v1alpha1/register.go to get new types within core group registered
	utilruntime.Must(corev1alpha1.AddToScheme(scheme))

	return (*Scheme)(scheme)
}
