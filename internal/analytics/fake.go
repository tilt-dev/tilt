package analytics

import (
	"sync"
	"testing"
	"time"

	"github.com/windmilleng/wmclient/pkg/analytics"
)

type FakeOpter struct {
	calls []analytics.Opt
	mu    sync.Mutex
}

func (to *FakeOpter) SetOpt(opt analytics.Opt) error {
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
