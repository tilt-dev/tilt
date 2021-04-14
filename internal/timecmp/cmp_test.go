package timecmp

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type cmpFunc func(a, b commonTime) bool

type tc struct {
	x   time.Time
	y   time.Time
	cmp cmpFunc

	second      bool
	microsecond bool
	nanosecond  bool
}

func TestEqual(t *testing.T) {
	reflexiveTest(t, Equal)

	t.Run("NanosecondDifference", func(t *testing.T) {
		// truncate to avoid potential roll-over
		now := time.Now().Truncate(time.Nanosecond)

		runTest(t, tc{
			x:           now,
			y:           now.Add(time.Nanosecond),
			cmp:         Equal,
			nanosecond:  false,
			microsecond: true,
			second:      true,
		})
	})

	t.Run("MicrosecondDifference", func(t *testing.T) {
		// truncate to avoid potential roll-over
		now := time.Now().Truncate(time.Microsecond)

		runTest(t, tc{
			x:           now,
			y:           now.Add(time.Microsecond),
			cmp:         Equal,
			nanosecond:  false,
			microsecond: false,
			second:      true,
		})
	})

	t.Run("SecondDifference", func(t *testing.T) {
		// truncate to avoid potential roll-over
		now := time.Now().Truncate(time.Second)

		runTest(t, tc{
			x:           now,
			y:           now.Add(time.Second),
			cmp:         Equal,
			nanosecond:  false,
			microsecond: false,
			second:      false,
		})
	})
}

func TestBeforeOrEqual(t *testing.T) {
	reflexiveTest(t, BeforeOrEqual)

	t.Run("NanosecondDifference", func(t *testing.T) {
		a := time.Now().Truncate(time.Nanosecond).Add(-5 * time.Nanosecond)
		b := a.Add(time.Nanosecond)

		// x before y
		runTest(t, tc{
			x:           a,
			y:           b,
			cmp:         BeforeOrEqual,
			nanosecond:  true,
			microsecond: true,
			second:      true,
		})

		// x after y
		runTest(t, tc{
			x:           b,
			y:           a,
			cmp:         BeforeOrEqual,
			nanosecond:  false,
			microsecond: true,
			second:      true,
		})
	})

	t.Run("MicrosecondDifference", func(t *testing.T) {
		a := time.Now().Truncate(time.Microsecond).Add(-5 * time.Microsecond)
		b := a.Add(time.Microsecond)

		// x before y
		runTest(t, tc{
			x:           a,
			y:           b,
			cmp:         BeforeOrEqual,
			nanosecond:  true,
			microsecond: true,
			second:      true,
		})

		// x after y
		runTest(t, tc{
			x:           b,
			y:           a,
			cmp:         BeforeOrEqual,
			nanosecond:  false,
			microsecond: false,
			second:      true,
		})
	})

	t.Run("SecondDifference", func(t *testing.T) {
		a := time.Now().Truncate(time.Second).Add(-5 * time.Second)
		b := a.Add(time.Second)

		// x before y
		runTest(t, tc{
			x:           a,
			y:           b,
			cmp:         BeforeOrEqual,
			nanosecond:  true,
			microsecond: true,
			second:      true,
		})

		// x after y
		runTest(t, tc{
			x:           b,
			y:           a,
			cmp:         BeforeOrEqual,
			nanosecond:  false,
			microsecond: false,
			second:      false,
		})
	})
}

func TestAfterOrEqual(t *testing.T) {
	reflexiveTest(t, AfterOrEqual)

	t.Run("NanosecondDifference", func(t *testing.T) {
		a := time.Now().Truncate(time.Nanosecond).Add(-5 * time.Nanosecond)
		b := a.Add(time.Nanosecond)

		// x before y
		runTest(t, tc{
			x:           a,
			y:           b,
			cmp:         AfterOrEqual,
			nanosecond:  false,
			microsecond: true,
			second:      true,
		})

		// x after y
		runTest(t, tc{
			x:           b,
			y:           a,
			cmp:         AfterOrEqual,
			nanosecond:  true,
			microsecond: true,
			second:      true,
		})
	})

	t.Run("MicrosecondDifference", func(t *testing.T) {
		a := time.Now().Truncate(time.Microsecond).Add(-5 * time.Microsecond)
		b := a.Add(time.Microsecond)

		// x before y
		runTest(t, tc{
			x:           a,
			y:           b,
			cmp:         AfterOrEqual,
			nanosecond:  false,
			microsecond: false,
			second:      true,
		})

		// x after y
		runTest(t, tc{
			x:           b,
			y:           a,
			cmp:         AfterOrEqual,
			nanosecond:  true,
			microsecond: true,
			second:      true,
		})
	})

	t.Run("SecondDifference", func(t *testing.T) {
		a := time.Now().Truncate(time.Second).Add(-5 * time.Second)
		b := a.Add(time.Second)

		// x before y
		runTest(t, tc{
			x:           a,
			y:           b,
			cmp:         AfterOrEqual,
			nanosecond:  false,
			microsecond: false,
			second:      false,
		})

		// x after y
		runTest(t, tc{
			x:           b,
			y:           a,
			cmp:         AfterOrEqual,
			nanosecond:  true,
			microsecond: true,
			second:      true,
		})
	})
}

func reflexiveTest(t *testing.T, cmp cmpFunc) {
	t.Run("Reflexive", func(t *testing.T) {
		now := time.Now()
		runTest(t, tc{
			x:           now,
			y:           now,
			cmp:         cmp,
			nanosecond:  true,
			microsecond: true,
			second:      true,
		})
	})
}

func runTest(t testing.TB, tc tc) {
	assert.Equal(t, tc.nanosecond, tc.cmp(tc.x, tc.y), "Nanosecond (stdlib <> stdlib) comparison failed")

	assert.Equal(t, tc.microsecond, tc.cmp(metav1.NewMicroTime(tc.x), metav1.NewMicroTime(tc.y)),
		"Microsecond (metav1.MicroTime <> metav1.MicroTime) comparison failed. Values:\n- x: %s\n- y: %s",
		tc.x.String(), tc.y.String())
	assert.Equal(t, tc.microsecond, tc.cmp(metav1.NewMicroTime(tc.x), tc.y),
		"Microsecond (metav1.MicroTime <> stdlib) comparison failed. Values:\n- x: %s\n- y: %s",
		tc.x.String(), tc.y.String())
	assert.Equal(t, tc.microsecond, tc.cmp(tc.x, metav1.NewMicroTime(tc.y)),
		"Microsecond (stdlib <> metav1.MicroTime) comparison failed. Values:\n- x: %s\n- y: %s",
		tc.x.String(), tc.y.String())

	assert.Equal(t, tc.second, tc.cmp(metav1.NewTime(tc.x), metav1.NewTime(tc.y)),
		"Second (metav1.Time <> metav1.Time) comparison failed. Values:\n- x: %s\n- y: %s",
		tc.x.String(), tc.y.String())
	assert.Equal(t, tc.second, tc.cmp(metav1.NewTime(tc.x), tc.y),
		"Second (metav1.Time <> stdlib) comparison failed. Values:\n- x: %s\n- y: %s",
		tc.x.String(), tc.y.String())
	assert.Equal(t, tc.second, tc.cmp(tc.x, metav1.NewTime(tc.y)),
		"Second (stdlib <> metav1.Time) comparison failed. Values:\n- x: %s\n- y: %s",
		tc.x.String(), tc.y.String())
	assert.Equal(t, tc.second, tc.cmp(metav1.NewTime(tc.x), metav1.NewMicroTime(tc.y)),
		"Second (metav1.Time <> metav1.MicroTime) comparison failed. Values:\n- x: %s\n- y: %s",
		tc.x.String(), tc.y.String())
	assert.Equal(t, tc.second, tc.cmp(metav1.NewMicroTime(tc.x), metav1.NewTime(tc.y)),
		"Second (metav1.MicroTime <> metav1.Time) comparison failed. Values:\n- x: %s\n- y: %s",
		tc.x.String(), tc.y.String())
}
