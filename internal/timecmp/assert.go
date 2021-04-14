package timecmp

import (
	"fmt"
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
func AssertTimeEqual(t testing.TB, expected stringableTimeValue, actual stringableTimeValue) bool {
	t.Helper()
	if !Equal(expected, actual) {
		return assert.Fail(t, fmt.Sprintf("Not equal: \n"+
			"expected: %s\n"+
			"actual  : %s", expected.String(), actual.String()))
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
