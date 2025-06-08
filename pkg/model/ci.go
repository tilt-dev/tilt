package model

import (
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/tilt-dev/tilt/pkg/apis/core/v1alpha1"
)

// Inject the flag-specified CI timeout.
type CITimeoutFlag time.Duration

const CITimeoutDefault = 30 * time.Minute

const CIReadinessTimeoutDefault = 5 * time.Minute

func DefaultSessionCISpec(ciTimeoutFlag CITimeoutFlag) *v1alpha1.SessionCISpec {
	return &v1alpha1.SessionCISpec{
		Timeout: &metav1.Duration{
			Duration: time.Duration(ciTimeoutFlag),
		},
		ReadinessTimeout: &metav1.Duration{
			Duration: CIReadinessTimeoutDefault,
		},
	}
}
