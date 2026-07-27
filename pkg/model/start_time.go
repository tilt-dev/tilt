package model

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/tilt-dev/tilt/pkg/apis"
)

// StartTime is the time Tilt started. It's the single source of truth for the
// "since" time on log streams (both Kubernetes pod logs and Docker Compose
// logs), replacing per-package package-initialization timestamps. It's injected
// via wire so there's exactly one value per Tilt process.
type StartTime metav1.Time

// ProvideStartTime captures the Tilt process start time once, at wire injection
// time.
func ProvideStartTime() StartTime {
	return StartTime(apis.Now())
}
