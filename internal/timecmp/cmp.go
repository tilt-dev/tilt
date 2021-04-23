// Package timecmp provides utility functions for comparing apiserver and stdlib times.
//
// There are a couple big pitfalls when using apiserver time types.
//
// First, the apiserver time types (metav1.Time & metav1.MicroTime) have second and microsecond
// granularity once serialized, respectively. Internally, however, they are wrappers around
// the Go stdlib times. As a result, initialized values that have not yet round-tripped to
// the server can have more granularity than they should.
//
// To address this issue, there are convenience constructors in tilt/pkg/apis that should be
// used for conversions from Go stdlib time types, including Now(). These are similar to the
// ones provided by metav1 itself except that they _immediately_ truncate.
//
// The second issue is addressed by this package, which is that internal timestamps within
// the Tilt engine often have higher granularity, which means comparisons can be problematic.
// For example, if an internal timestamp of an operation is held as a Go stdlib time.Time value
// and then stored on an entity as a metav1.Time object, future comparisons might not behave as
// expected since the latter value will be truncated.
//
// The comparison functions provided by this package normalize values to the lowest granularity
// of the values being compared before performing the actual comparison.
package timecmp

import (
	"fmt"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// commonTime is the common interface between apiserver + Go stdlib time types necessary for
// normalization.
type commonTime interface {
	Truncate(duration time.Duration) time.Time
}

// Equal returns true of the normalized versions of a and b are equal.
//
// Values are normalized to the lowest granularity between the two values: seconds if either
// is metav1.Time, microseconds if either is metav1.MicroTime, or monotonically-stripped if
// both are time.Time.
func Equal(a, b commonTime) bool {
	aNorm, bNorm := normalize(a, b)
	return aNorm.Equal(bNorm)
}

// BeforeOrEqual returns true if the normalized version of a is before or equal to the normalized version of b.
//
// Values are normalized to the lowest granularity between the two values: seconds if either
// is metav1.Time, microseconds if either is metav1.MicroTime, or monotonically-stripped if
// both are time.Time.
func BeforeOrEqual(a, b commonTime) bool {
	aNorm, bNorm := normalize(a, b)
	return aNorm.Before(bNorm) || aNorm.Equal(bNorm)
}

// AfterOrEqual returns true if the normalized version of a is after or equal to the normalized version of b.
//
// Values are normalized to the lowest granularity between the two values: seconds if either
// is metav1.Time, microseconds if either is metav1.MicroTime, or monotonically-stripped if
// both are time.Time.
func AfterOrEqual(a, b commonTime) bool {
	aNorm, bNorm := normalize(a, b)
	return aNorm.After(bNorm) || aNorm.Equal(bNorm)
}

// normalize returns versions of a and b truncated to the lowest available granularity.
//
// 	* If either is metav1.Time, a and b are truncated to time.Second.
// 	* If either is metav1.MicroTime, a and b are truncated to time.Microsecond.
// 	* If both a and b are time.Time, a and b have their monotonic clock reading stripped but are otherwise untouched.
// 	* Otherwise, this function will panic.
func normalize(a, b commonTime) (time.Time, time.Time) {
	var anySeconds bool
	var anyMicroseconds bool
	for _, x := range []commonTime{a, b} {
		switch x.(type) {
		case metav1.Time:
			anySeconds = true
		case *metav1.Time:
			anySeconds = true
		case metav1.MicroTime:
			anyMicroseconds = true
		case *metav1.MicroTime:
			anyMicroseconds = true
		// stdlib time is accepted, but has nanosecond-granularity, so nothing more to do
		case time.Time:
		case *time.Time:
		default:
			panic(fmt.Errorf("unexpected type for time normalization: %T", x))
		}
	}

	if anySeconds {
		return a.Truncate(time.Second), b.Truncate(time.Second)
	}

	if anyMicroseconds {
		return a.Truncate(time.Microsecond), b.Truncate(time.Microsecond)
	}

	// truncate with value <= 0 will strip off monotonic clock reading but
	// otherwise leave untouched; this is actually desirable because Windows
	// does not provide monotonically increasing clock readings, so this
	// reduces the likelihood of non-portable time logic being introduced
	return a.Truncate(0), b.Truncate(0)
}
