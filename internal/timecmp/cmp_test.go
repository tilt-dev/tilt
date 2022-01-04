package timecmp

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type cmpFunc func(a, b commonTime) bool
type timeFn func() *time.Time

type tc struct {
	x   *time.Time
	y   *time.Time
	cmp cmpFunc

	second      bool
	microsecond bool
	nanosecond  bool
}

func now() *time.Time {
	v := time.Unix(1619635910, 450240689)
	return &v
}

func TestEqual(t *testing.T) {
	reflexiveTest(t, Equal, now)

	t.Run("NanosecondDifference", func(t *testing.T) {
		// truncate to avoid potential roll-over
		now := now().Truncate(time.Nanosecond)
		x := now
		y := now.Add(time.Nanosecond)

		runTest(t, tc{
			x:           &x,
			y:           &y,
			cmp:         Equal,
			nanosecond:  false,
			microsecond: true,
			second:      true,
		})
	})

	t.Run("MicrosecondDifference", func(t *testing.T) {
		// truncate to avoid potential roll-over
		now := now().Truncate(time.Microsecond)
		x := now
		y := now.Add(time.Microsecond)

		runTest(t, tc{
			x:           &x,
			y:           &y,
			cmp:         Equal,
			nanosecond:  false,
			microsecond: false,
			second:      true,
		})
	})

	t.Run("SecondDifference", func(t *testing.T) {
		// truncate to avoid potential roll-over
		now := now().Truncate(time.Second)
		x := now
		y := now.Add(time.Second)

		runTest(t, tc{
			x:           &x,
			y:           &y,
			cmp:         Equal,
			nanosecond:  false,
			microsecond: false,
			second:      false,
		})
	})

	t.Run("Nil", func(t *testing.T) {
		nilTime := func() *time.Time {
			return nil
		}

		reflexiveTest(t, Equal, nilTime)

		t.Run("X", func(t *testing.T) {
			// use a typed nil to mimic K8s API objects
			x := (*time.Time)(nil)
			y := now()
			runTest(t, tc{
				x:           x,
				y:           y,
				cmp:         Equal,
				nanosecond:  false,
				microsecond: false,
				second:      false,
			})
		})

		t.Run("Y", func(t *testing.T) {
			x := now()
			runTest(t, tc{
				x:           x,
				y:           nil,
				cmp:         Equal,
				nanosecond:  false,
				microsecond: false,
				second:      false,
			})
		})
	})
}

func TestBeforeOrEqual(t *testing.T) {
	reflexiveTest(t, BeforeOrEqual, now)

	t.Run("NanosecondDifference", func(t *testing.T) {
		a := now().Truncate(time.Nanosecond).Add(-5 * time.Nanosecond)
		b := a.Add(time.Nanosecond)

		// x before y
		runTest(t, tc{
			x:           &a,
			y:           &b,
			cmp:         BeforeOrEqual,
			nanosecond:  true,
			microsecond: true,
			second:      true,
		})

		// x after y
		runTest(t, tc{
			x:           &b,
			y:           &a,
			cmp:         BeforeOrEqual,
			nanosecond:  false,
			microsecond: true,
			second:      true,
		})
	})

	t.Run("MicrosecondDifference", func(t *testing.T) {
		a := now().Truncate(time.Microsecond).Add(-5 * time.Microsecond)
		b := a.Add(time.Microsecond)

		// x before y
		runTest(t, tc{
			x:           &a,
			y:           &b,
			cmp:         BeforeOrEqual,
			nanosecond:  true,
			microsecond: true,
			second:      true,
		})

		// x after y
		runTest(t, tc{
			x:           &b,
			y:           &a,
			cmp:         BeforeOrEqual,
			nanosecond:  false,
			microsecond: false,
			second:      true,
		})
	})

	t.Run("SecondDifference", func(t *testing.T) {
		a := now().Truncate(time.Second).Add(-5 * time.Second)
		b := a.Add(time.Second)

		// x before y
		runTest(t, tc{
			x:           &a,
			y:           &b,
			cmp:         BeforeOrEqual,
			nanosecond:  true,
			microsecond: true,
			second:      true,
		})

		// x after y
		runTest(t, tc{
			x:           &b,
			y:           &a,
			cmp:         BeforeOrEqual,
			nanosecond:  false,
			microsecond: false,
			second:      false,
		})
	})
}

func TestAfterOrEqual(t *testing.T) {
	reflexiveTest(t, AfterOrEqual, now)

	t.Run("NanosecondDifference", func(t *testing.T) {
		a := now().Truncate(time.Nanosecond).Add(-5 * time.Nanosecond)
		b := a.Add(time.Nanosecond)

		// x before y
		runTest(t, tc{
			x:           &a,
			y:           &b,
			cmp:         AfterOrEqual,
			nanosecond:  false,
			microsecond: true,
			second:      true,
		})

		// x after y
		runTest(t, tc{
			x:           &b,
			y:           &a,
			cmp:         AfterOrEqual,
			nanosecond:  true,
			microsecond: true,
			second:      true,
		})
	})

	t.Run("MicrosecondDifference", func(t *testing.T) {
		a := now().Truncate(time.Microsecond).Add(-5 * time.Microsecond)
		b := a.Add(time.Microsecond)

		// x before y
		runTest(t, tc{
			x:           &a,
			y:           &b,
			cmp:         AfterOrEqual,
			nanosecond:  false,
			microsecond: false,
			second:      true,
		})

		// x after y
		runTest(t, tc{
			x:           &b,
			y:           &a,
			cmp:         AfterOrEqual,
			nanosecond:  true,
			microsecond: true,
			second:      true,
		})
	})

	t.Run("SecondDifference", func(t *testing.T) {
		a := now().Truncate(time.Second).Add(-5 * time.Second)
		b := a.Add(time.Second)

		// x before y
		runTest(t, tc{
			x:           &a,
			y:           &b,
			cmp:         AfterOrEqual,
			nanosecond:  false,
			microsecond: false,
			second:      false,
		})

		// x after y
		runTest(t, tc{
			x:           &b,
			y:           &a,
			cmp:         AfterOrEqual,
			nanosecond:  true,
			microsecond: true,
			second:      true,
		})
	})
}

func reflexiveTest(t *testing.T, cmp cmpFunc, timeVal timeFn) {
	t.Run("Reflexive", func(t *testing.T) {
		v := timeVal()
		runTest(t, tc{
			x:           v,
			y:           v,
			cmp:         cmp,
			nanosecond:  true,
			microsecond: true,
			second:      true,
		})
	})
}

func runTest(t testing.TB, tc tc) {
	assert.Equal(t, tc.nanosecond, tc.cmp(tc.x, tc.y), "Nanosecond (stdlib <> stdlib) comparison failed")

	assert.Equal(t, tc.microsecond, tc.cmp(apiMicroTime(tc.x), apiMicroTime(tc.y)),
		"Microsecond (metav1.MicroTime <> metav1.MicroTime) comparison failed. Values:\n- x: %v\n- y: %v",
		tc.x, tc.y)
	assert.Equal(t, tc.microsecond, tc.cmp(apiMicroTime(tc.x), tc.y),
		"Microsecond (metav1.MicroTime <> stdlib) comparison failed. Values:\n- x: %v\n- y: %v",
		tc.x, tc.y)
	assert.Equal(t, tc.microsecond, tc.cmp(tc.x, apiMicroTime(tc.y)),
		"Microsecond (stdlib <> metav1.MicroTime) comparison failed. Values:\n- x: %v\n- y: %v",
		tc.x, tc.y)

	assert.Equal(t, tc.second, tc.cmp(apiTime(tc.x), apiTime(tc.y)),
		"Second (metav1.Time <> metav1.Time) comparison failed. Values:\n- x: %v\n- y: %v",
		tc.x, tc.y)
	assert.Equal(t, tc.second, tc.cmp(apiTime(tc.x), tc.y),
		"Second (metav1.Time <> stdlib) comparison failed. Values:\n- x: %v\n- y: %v",
		tc.x, tc.y)
	assert.Equal(t, tc.second, tc.cmp(tc.x, apiTime(tc.y)),
		"Second (stdlib <> metav1.Time) comparison failed. Values:\n- x: %v\n- y: %v",
		tc.x, tc.y)
	assert.Equal(t, tc.second, tc.cmp(apiTime(tc.x), apiMicroTime(tc.y)),
		"Second (metav1.Time <> metav1.MicroTime) comparison failed. Values:\n- x: %v\n- y: %v",
		tc.x, tc.y)
	assert.Equal(t, tc.second, tc.cmp(apiMicroTime(tc.x), apiTime(tc.y)),
		"Second (metav1.MicroTime <> metav1.Time) comparison failed. Values:\n- x: %v\n- y: %v",
		tc.x, tc.y)

	// pointer test cases (non-exhaustive)
	xAPITime := apiTimeP(tc.x)
	yMicroTime := apiMicroTimeP(tc.y)
	assert.Equal(t, tc.second, tc.cmp(xAPITime, yMicroTime),
		"Second (*metav1.Time <> *metav1.MicroTime) comparison failed. Values:\n- x: %v\n- y: %v",
		tc.x, tc.y)
	assert.Equal(t, tc.second, tc.cmp(xAPITime, tc.y),
		"Second (metav1.Time <> *stdlib) comparison failed. Values:\n- x: %v\n- y: %v",
		tc.x, tc.y)
}

func apiMicroTime(v *time.Time) commonTime {
	if v == nil {
		return nil
	}
	return metav1.NewMicroTime(*v)
}

func apiMicroTimeP(v *time.Time) *metav1.MicroTime {
	if v == nil {
		return nil
	}
	t := metav1.NewMicroTime(*v)
	return &t
}

func apiTime(v *time.Time) commonTime {
	if v == nil {
		return nil
	}
	return metav1.NewTime(*v)
}

func apiTimeP(v *time.Time) *metav1.Time {
	if v == nil {
		return nil
	}
	t := metav1.NewTime(*v)
	return &t
}
