package apis

import (
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// NewMicroTime returns a wrapped instance of the provided time truncated to microseconds.
func NewMicroTime(v time.Time) metav1.MicroTime {
	return metav1.NewMicroTime(v.Truncate(time.Microsecond))
}

// NewTime returns a wrapped instance of the provided time truncated to seconds.
func NewTime(v time.Time) metav1.Time {
	return metav1.NewTime(v.Truncate(time.Second))
}

// Now returns the current local time truncated to seconds.
func Now() metav1.Time {
	return NewTime(time.Now())
}

// NowMicro returns the current local time truncated to microseconds.
func NowMicro() metav1.MicroTime {
	return NewMicroTime(time.Now())
}
