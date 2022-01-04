package timecmp

import (
	"fmt"
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
)

type stringableTimeValue interface {
	commonTime
	String() string
}

// AssertTimeEqual compares two time values using timecmp.Equal and fails the test if not equal.
//
// This simplifies comparing Go stdlib time.Time values with apiserver metav1.Time / metav1.MicroTime values
// based on the minimum granularity between the two values.
func AssertTimeEqual(t testing.TB, expected stringableTimeValue, actual stringableTimeValue) (equal bool) {
	t.Helper()

	defer func() {
		if !equal {
			assert.Fail(t, fmt.Sprintf("Not equal: \n"+
				"expected: %s\n"+
				"actual  : %s", expected, actual))
		}
	}()

	expectedNil := isNilValue(expected)
	actualNil := isNilValue(actual)
	if expectedNil && actualNil {
		return true
	} else if expectedNil || actualNil {
		return false
	}

	if !Equal(expected, actual) {
		return false
	}
	return true
}

// RequireTimeEqual compares two time values using timecmp.Equal and fails the test immediately if not equal.
//
// This simplifies comparing Go stdlib time.Time values with apiserver metav1.Time / metav1.MicroTime values
// based on the minimum granularity between the two values.
func RequireTimeEqual(t testing.TB, expected stringableTimeValue, actual stringableTimeValue) {
	t.Helper()
	if !AssertTimeEqual(t, expected, actual) {
		t.FailNow()
	}
}

// K8s types will come back with typed nils, so normal comparisons won't
// work; since this is purely for tests, reflection is easiest option
func isNilValue(t stringableTimeValue) bool {
	if t == nil {
		return true
	}
	v := reflect.ValueOf(t)
	if v.Kind() == reflect.Ptr {
		return v.IsNil()
	}
	return false
}
