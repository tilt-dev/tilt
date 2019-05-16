package analytics

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/windmilleng/wmclient/pkg/analytics"
)

type testCase struct {
	name         string
	opt          analytics.Opt
	expectRecord bool
}

func testCases(expectedWhenOptedIn, expectedWhenOptedOut, expectedWhenNoOpt bool) []testCase {
	return []testCase{
		{"opt in", analytics.OptIn, expectedWhenOptedIn},
		{"opt out", analytics.OptOut, expectedWhenOptedOut},
		{"opt default", analytics.OptDefault, expectedWhenNoOpt},
	}
}

type optSetting struct {
	opt analytics.Opt
}

func (os *optSetting) setOpt(opt analytics.Opt) error {
	os.opt = opt
	return nil
}

func TestCount(t *testing.T) {
	for _, test := range testCases(true, false, false) {
		t.Run(test.name, func(t *testing.T) {
			ma := analytics.NewMemoryAnalytics()
			os := &optSetting{}
			a := NewTiltAnalytics(test.opt, os.setOpt, ma)
			a.Count("foo", map[string]string{}, 1)
			var expectedCounts []analytics.CountEvent
			if test.expectRecord {
				expectedCounts = append(expectedCounts, analytics.CountEvent{
					Name: "foo",
					Tags: map[string]string{},
					N:    1,
				})
			}
			assert.Equal(t, expectedCounts, ma.Counts)
		})
	}
}

func TestIncr(t *testing.T) {
	for _, test := range testCases(true, false, false) {
		t.Run(test.name, func(t *testing.T) {
			ma := analytics.NewMemoryAnalytics()
			os := &optSetting{}
			a := NewTiltAnalytics(test.opt, os.setOpt, ma)
			a.Incr("foo", map[string]string{})
			var expectedCounts []analytics.CountEvent
			if test.expectRecord {
				expectedCounts = append(expectedCounts, analytics.CountEvent{
					Name: "foo",
					Tags: map[string]string{},
					N:    1,
				})
			}
			assert.Equal(t, expectedCounts, ma.Counts)
		})
	}
}

func TestTimer(t *testing.T) {
	for _, test := range testCases(true, false, false) {
		t.Run(test.name, func(t *testing.T) {
			ma := analytics.NewMemoryAnalytics()
			os := &optSetting{}
			a := NewTiltAnalytics(test.opt, os.setOpt, ma)
			a.Timer("foo", time.Second, map[string]string{})
			var expectedTimes []analytics.TimeEvent
			if test.expectRecord {
				expectedTimes = append(expectedTimes, analytics.TimeEvent{
					Name: "foo",
					Tags: map[string]string{},
					Dur:  time.Second,
				})
			}
			assert.Equal(t, expectedTimes, ma.Timers)
		})
	}
}

func TestIncrIfUnopted(t *testing.T) {
	for _, test := range testCases(false, false, true) {
		t.Run(test.name, func(t *testing.T) {
			ma := analytics.NewMemoryAnalytics()
			os := &optSetting{}
			a := NewTiltAnalytics(test.opt, os.setOpt, ma)
			a.IncrIfUnopted("foo", map[string]string{})
			var expectedCounts []analytics.CountEvent
			if test.expectRecord {
				expectedCounts = append(expectedCounts, analytics.CountEvent{
					Name: "foo",
					Tags: map[string]string{},
					N:    1,
				})
			}
			assert.Equal(t, expectedCounts, ma.Counts)
		})
	}
}

func analyticsViaTransition(t *testing.T, initialOpt, newOpt analytics.Opt) (*TiltAnalytics, *analytics.MemoryAnalytics) {
	ma := analytics.NewMemoryAnalytics()
	os := optSetting{}
	a := NewTiltAnalytics(initialOpt, os.setOpt, ma)
	switch newOpt {
	case analytics.OptIn:
		err := a.OptIn()
		if err != nil {
			assert.FailNow(t, err.Error())
		}
	case analytics.OptOut:
		err := a.OptOut()
		if err != nil {
			assert.FailNow(t, err.Error())
		}
	}

	if !assert.Equal(t, newOpt, os.opt) {
		t.FailNow()
	}

	// wipe out the reports of opt-in/out, since those are just setup for another test
	ma.Counts = nil

	return a, ma
}

type transitionTestCase struct {
	name         string
	initialOpt   analytics.Opt
	newOpt       analytics.Opt
	expectRecord bool
}

func TestOptTransitionIncr(t *testing.T) {
	for _, test := range []transitionTestCase{
		{"default -> out", analytics.OptDefault, analytics.OptOut, false},
		{"default -> in", analytics.OptDefault, analytics.OptIn, true},
		{"in -> out", analytics.OptIn, analytics.OptOut, false},
		{"out -> in", analytics.OptOut, analytics.OptIn, true},
	} {
		t.Run(test.name, func(t *testing.T) {
			a, ma := analyticsViaTransition(t, test.initialOpt, test.newOpt)
			a.Incr("foo", map[string]string{})
			var expectedCounts []analytics.CountEvent
			if test.expectRecord {
				expectedCounts = append(expectedCounts, analytics.CountEvent{
					Name: "foo",
					Tags: map[string]string{},
					N:    1,
				})
			}
			assert.Equal(t, expectedCounts, ma.Counts)
		})
	}
}

func TestOptTransitionIncrIfUnopted(t *testing.T) {
	for _, test := range []transitionTestCase{
		{"default -> out", analytics.OptDefault, analytics.OptOut, false},
		{"default -> in", analytics.OptDefault, analytics.OptIn, false},
		{"in -> out", analytics.OptIn, analytics.OptOut, false},
		{"out -> in", analytics.OptOut, analytics.OptIn, false},
	} {
		t.Run(test.name, func(t *testing.T) {
			a, ma := analyticsViaTransition(t, test.initialOpt, test.newOpt)
			a.IncrIfUnopted("foo", map[string]string{})
			var expectedCounts []analytics.CountEvent
			if test.expectRecord {
				expectedCounts = append(expectedCounts, analytics.CountEvent{
					Name: "foo",
					Tags: map[string]string{},
					N:    1,
				})
			}
			assert.Equal(t, expectedCounts, ma.Counts)
		})
	}
}

func TestOptIn(t *testing.T) {
	for _, test := range []struct {
		name       string
		f          func(a *TiltAnalytics) error
		metricName string
	}{
		{"in", func(a *TiltAnalytics) error { return a.OptIn() }, "analytics.opt.in"},
		{"out", func(a *TiltAnalytics) error { return a.OptOut() }, ""},
	} {
		t.Run(test.name, func(t *testing.T) {
			ma := analytics.NewMemoryAnalytics()
			os := &optSetting{}
			a := NewTiltAnalytics(analytics.OptDefault, os.setOpt, ma)
			err := test.f(a)
			if !assert.NoError(t, err) {
				t.FailNow()
			}

			var expectedCounts []analytics.CountEvent
			if test.metricName != "" {
				expectedCounts = append(expectedCounts, analytics.CountEvent{
					Name: test.metricName,
					Tags: map[string]string{},
					N:    1,
				})
			}
			assert.Equal(t, expectedCounts, ma.Counts)
		})
	}
}
