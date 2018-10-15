package engine

import (
	"context"

	"github.com/windmilleng/tilt/internal/store"

	"github.com/windmilleng/tilt/internal/k8s"
	"k8s.io/api/core/v1"
)

func makeServiceWatcher(ctx context.Context, kCli k8s.Client, st *store.Store) error {
	ch, err := kCli.WatchServices(ctx, []k8s.LabelPair{TiltRunLabel()})
	if err != nil {
		return err
	}

	go dispatchServiceChangesLoop(ctx, ch, st)

	return nil
}

func dispatchServiceChangesLoop(ctx context.Context, ch <-chan *v1.Service, st *store.Store) {
	for {
		select {
		case service, ok := <-ch:
			if !ok {
				return
			}
			st.Dispatch(NewServiceChangeAction(service))
		case <-ctx.Done():
			return
		}
	}
}
