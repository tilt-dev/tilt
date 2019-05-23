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

var testTags = map[string]string{"bar": "baz"}

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

func (os *optSetting) SetOpt(opt analytics.Opt) error {
	os.opt = opt
	return nil
}

func TestCount(t *testing.T) {
	for _, test := range testCases(true, false, false) {
		t.Run(test.name, func(t *testing.T) {
			ma := analytics.NewMemoryAnalytics()
			os := &optSetting{}
			a := NewTiltAnalytics(test.opt, os, ma)
			a.Count("foo", testTags, 1)
			var expectedCounts []analytics.CountEvent
			if test.expectRecord {
				expectedCounts = append(expectedCounts, analytics.CountEvent{
					Name: "foo",
					Tags: testTags,
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
			a := NewTiltAnalytics(test.opt, os, ma)
			a.Incr("foo", testTags)
			var expectedCounts []analytics.CountEvent
			if test.expectRecord {
				expectedCounts = append(expectedCounts, analytics.CountEvent{
					Name: "foo",
					Tags: testTags,
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
			a := NewTiltAnalytics(test.opt, os, ma)
			a.Timer("foo", time.Second, testTags)
			var expectedTimes []analytics.TimeEvent
			if test.expectRecord {
				expectedTimes = append(expectedTimes, analytics.TimeEvent{
					Name: "foo",
					Tags: testTags,
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
			a := NewTiltAnalytics(test.opt, os, ma)
			a.IncrIfUnopted("foo")
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
	os := &optSetting{}
	a := NewTiltAnalytics(initialOpt, os, ma)
	err := a.SetOpt(newOpt)
	if !assert.NoError(t, err) {
		assert.FailNow(t, err.Error())
	}

	if !assert.Equal(t, newOpt, os.opt) {
		t.FailNow()
	}

	// wipe out the reports of opt-in/out, since those are side effects of test setup, not the test itself
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
			a.Incr("foo", testTags)
			var expectedCounts []analytics.CountEvent
			if test.expectRecord {
				expectedCounts = append(expectedCounts, analytics.CountEvent{
					Name: "foo",
					Tags: testTags,
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
			a.IncrIfUnopted("foo")
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
		opt        analytics.Opt
		metricName string
	}{
		{"in", analytics.OptIn, "analytics.opt.in"},
		{"out", analytics.OptOut, ""},
	} {
		t.Run(test.name, func(t *testing.T) {
			ma := analytics.NewMemoryAnalytics()
			os := &optSetting{}
			a := NewTiltAnalytics(analytics.OptDefault, os, ma)
			err := a.SetOpt(test.opt)
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

type testOpter struct {
	calls []analytics.Opt
}

func (t *testOpter) SetOpt(opt analytics.Opt) error {
	t.calls = append(t.calls, opt)
	return nil
}

var _ AnalyticsOpter = &testOpter{}
