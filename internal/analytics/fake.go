package analytics

import (
	"sync"
	"testing"
	"time"

	"github.com/windmilleng/wmclient/pkg/analytics"
)

type FakeOpter struct {
	initialOpt analytics.Opt
	calls      []analytics.Opt
	mu         sync.Mutex
}

func NewFakeOpter(initialOpt analytics.Opt) *FakeOpter {
	return &FakeOpter{initialOpt: initialOpt}
}

func DefaultFakeOpter() *FakeOpter {
	return NewFakeOpter(analytics.OptDefault)
}

func (to *FakeOpter) ReadUserOpt() (analytics.Opt, error) {
	to.mu.Lock()
	defer to.mu.Unlock()
	if len(to.calls) == 0 {
		return to.initialOpt, nil
	}
	return to.calls[len(to.calls)-1], nil
}

func (to *FakeOpter) SetUserOpt(opt analytics.Opt) error {
	to.mu.Lock()
	defer to.mu.Unlock()
	to.calls = append(to.calls, opt)
	return nil
}

func (to *FakeOpter) Calls() []analytics.Opt {
	to.mu.Lock()
	defer to.mu.Unlock()
	return append([]analytics.Opt{}, to.calls...)
}

func (to *FakeOpter) WaitUntilCount(t *testing.T, expectedCount int) {
	timeout := time.After(time.Second)
	for {
		select {
		case <-time.After(5 * time.Millisecond):
			actualCount := len(to.Calls())
			if actualCount == expectedCount {
				return
			}
		case <-timeout:
			actualCount := len(to.Calls())
			t.Errorf("waiting for opt setting count to be %d. opt setting count is currently %d", expectedCount, actualCount)
			t.FailNow()
		}
	}
}

var _ AnalyticsOpter = &FakeOpter{}
