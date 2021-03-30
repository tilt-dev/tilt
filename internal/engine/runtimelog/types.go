package runtimelog

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/tilt-dev/tilt/pkg/apis/core/v1alpha1"
)

type PodLogStream = v1alpha1.PodLogStream
type PodLogStreamSpec = v1alpha1.PodLogStreamSpec
type ObjectMeta = metav1.ObjectMeta
