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
	"reflect"
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
// both are time.Time. Nil time values are normalized to the zero-time.
func Equal(a, b commonTime) bool {
	aNorm, bNorm := normalize(a, b)
	return aNorm.Equal(bNorm)
}

// Before returns true if the normalized version of a is strictly before the normalized version of b.
//
// Values are normalized to the lowest granularity between the two values: seconds if either
// is metav1.Time, microseconds if either is metav1.MicroTime, or monotonically-stripped if
// both are time.Time. Nil time values are normalized to the zero-time.
func Before(a, b commonTime) bool {
	aNorm, bNorm := normalize(a, b)
	return aNorm.Before(bNorm)
}

// BeforeOrEqual returns true if the normalized version of a is before or equal to the normalized version of b.
//
// Values are normalized to the lowest granularity between the two values: seconds if either
// is metav1.Time, microseconds if either is metav1.MicroTime, or monotonically-stripped if
// both are time.Time. Nil time values are normalized to the zero-time.
func BeforeOrEqual(a, b commonTime) bool {
	aNorm, bNorm := normalize(a, b)
	return aNorm.Before(bNorm) || aNorm.Equal(bNorm)
}

// After returns true if the normalized version of a is strictly after the normalized version of b.
//
// Values are normalized to the lowest granularity between the two values: seconds if either
// is metav1.Time, microseconds if either is metav1.MicroTime, or monotonically-stripped if
// both are time.Time. Nil time values are normalized to the zero-time.
func After(a, b commonTime) bool {
	aNorm, bNorm := normalize(a, b)
	return aNorm.After(bNorm)
}

// AfterOrEqual returns true if the normalized version of a is after or equal to the normalized version of b.
//
// Values are normalized to the lowest granularity between the two values: seconds if either
// is metav1.Time, microseconds if either is metav1.MicroTime, or monotonically-stripped if
// both are time.Time. Nil time values are normalized to the zero-time.
func AfterOrEqual(a, b commonTime) bool {
	aNorm, bNorm := normalize(a, b)
	return aNorm.After(bNorm) || aNorm.Equal(bNorm)
}

// normalize returns versions of a and b truncated to the lowest available granularity.
//
//   - If either is metav1.Time, a and b are truncated to time.Second.
//   - If either is metav1.MicroTime, a and b are truncated to time.Microsecond.
//   - If both a and b are time.Time, a and b have their monotonic clock reading stripped but are otherwise untouched.
//   - If either is nil, nil value(s) are converted to the zero time and the non-nil value (if present) has the
//     monotonic clock reading stripped.
//   - Otherwise, this function will panic.
func normalize(a, b commonTime) (time.Time, time.Time) {
	var anySeconds bool
	var anyMicroseconds bool
	for _, x := range []commonTime{a, b} {
		switch x.(type) {
		case metav1.Time, *metav1.Time:
			anySeconds = true
		case metav1.MicroTime, *metav1.MicroTime:
			anyMicroseconds = true
		// stdlib time is accepted, but has nanosecond-granularity, so nothing more to do
		case time.Time, *time.Time:
		case nil:
			// coerce nils to zero time or strip off monotonic clock reading,
			// granularity isn't important since at least one value is nil
			return truncate(a, 0), truncate(b, 0)
		default:
			panic(fmt.Errorf("unexpected type for time normalization: %T", x))
		}
	}

	if anySeconds {
		return truncate(a, time.Second), truncate(b, time.Second)
	}

	if anyMicroseconds {
		return truncate(a, time.Microsecond), truncate(b, time.Microsecond)
	}

	// truncate with value <= 0 will strip off monotonic clock reading but
	// otherwise leave untouched; this is actually desirable because Windows
	// does not provide monotonically increasing clock readings, so this
	// reduces the likelihood of non-portable time logic being introduced
	return truncate(a, 0), truncate(b, 0)
}

func isNil(v commonTime) bool {
	if v == nil {
		return true
	}

	// K8s types will come back with typed nils, so we need to use reflection
	// to handle them properly
	x := reflect.ValueOf(v)
	if x.Kind() == reflect.Ptr && x.IsNil() {
		return true
	}

	return false
}

func truncate(v commonTime, d time.Duration) time.Time {
	if isNil(v) {
		return time.Time{}
	}
	return v.Truncate(d)
}
