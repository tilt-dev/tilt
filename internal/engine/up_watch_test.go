package engine

import (
	"context"
	"testing"

	"github.com/windmilleng/tilt/internal/testutils"
	"github.com/windmilleng/tilt/internal/watch"
)

func TestServiceWatcher(t *testing.T) {
	//	tf := makeServiceWatcherTestFixture()
	//	sw := makeServiceWatcher(tf.ctx, tf.watcherMaker, nil, services)

}

type serviceWatcherTestFixture struct {
	watcherMaker watcherMaker
	ctx          context.Context
}

func makeServiceWatcherTestFixture() *serviceWatcherTestFixture {
	watcher := newFakeNotify()
	watcherMaker := func() (watch.Notify, error) {
		return watcher, nil
	}

	ctx := testutils.CtxForTest()
	return &serviceWatcherTestFixture{watcherMaker, ctx}
}
