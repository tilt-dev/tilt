package analytics

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/windmilleng/wmclient/pkg/analytics"
)

const versionTest = "v0.0.0"

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

type userOptSetting struct {
	opt analytics.Opt
}

func (os *userOptSetting) ReadUserOpt() (analytics.Opt, error) {
	return os.opt, nil
}

func (os *userOptSetting) SetUserOpt(opt analytics.Opt) error {
	os.opt = opt
	return nil
}

func TestCount(t *testing.T) {
	for _, test := range testCases(true, false, true) {
		t.Run(test.name, func(t *testing.T) {
			ma := analytics.NewMemoryAnalytics()
			os := &userOptSetting{opt: test.opt}
			a, _ := NewTiltAnalytics(os, ma, versionTest)
			a.opt.env = analytics.OptDefault
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
	for _, test := range testCases(true, false, true) {
		t.Run(test.name, func(t *testing.T) {
			ma := analytics.NewMemoryAnalytics()
			os := &userOptSetting{opt: test.opt}
			a, _ := NewTiltAnalytics(os, ma, versionTest)
			a.opt.env = analytics.OptDefault
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
	for _, test := range testCases(true, false, true) {
		t.Run(test.name, func(t *testing.T) {
			ma := analytics.NewMemoryAnalytics()
			os := &userOptSetting{opt: test.opt}
			a, _ := NewTiltAnalytics(os, ma, versionTest)
			a.opt.env = analytics.OptDefault
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

func TestWithoutGlobalTags(t *testing.T) {
	ma := analytics.NewMemoryAnalytics()
	os := &userOptSetting{opt: analytics.OptIn}
	a, _ := NewTiltAnalytics(os, ma, versionTest)
	a.opt.env = analytics.OptDefault
	a.WithoutGlobalTags().Incr("foo", testTags)

	// memory analytics doesn't have global tags, so there's really
	// nothing to test. We mainly want to make sure this doesn't crash.
	assert.Equal(t, 1, len(ma.Counts))
}

func analyticsViaTransition(t *testing.T, initialOpt, newOpt analytics.Opt) (*TiltAnalytics, *analytics.MemoryAnalytics) {
	ma := analytics.NewMemoryAnalytics()
	os := &userOptSetting{opt: initialOpt}
	a, _ := NewTiltAnalytics(os, ma, versionTest)
	a.opt.env = analytics.OptDefault
	err := a.SetUserOpt(newOpt)
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
