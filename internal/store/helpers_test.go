package store

import (
	"context"
	"testing"
	"time"

	"github.com/tilt-dev/tilt/pkg/model"
)

func newState(manifests []model.Manifest) *EngineState {
	ret := NewState()
	for _, m := range manifests {
		ret.ManifestTargets[m.Name] = NewManifestTarget(m)
		ret.ManifestDefinitionOrder = append(ret.ManifestDefinitionOrder, m.Name)
	}

	return ret
}

type fakeSubscriber struct {
	onChange      chan onChangeCall
	setupCount    int
	teardownCount int
}

func newFakeSubscriber() *fakeSubscriber {
	return &fakeSubscriber{
		onChange: make(chan onChangeCall),
	}
}

type onChangeCall struct {
	done chan bool
}

func (f *fakeSubscriber) assertOnChangeCount(t *testing.T, count int) {
	t.Helper()

	for i := 0; i < count; i++ {
		f.assertOnChange(t)
	}

	select {
	case <-time.After(50 * time.Millisecond):
		return

	case call := <-f.onChange:
		close(call.done)
		t.Fatalf("Expected only %d OnChange calls. Got: %d", count, count+1)
	}
}

func (f *fakeSubscriber) assertOnChange(t *testing.T) {
	t.Helper()

	select {
	case <-time.After(50 * time.Millisecond):
		t.Fatalf("timed out waiting for subscriber.OnChange")
	case call := <-f.onChange:
		close(call.done)
	}
}

func (f *fakeSubscriber) OnChange(ctx context.Context, st RStore) {
	call := onChangeCall{done: make(chan bool)}
	f.onChange <- call
	<-call.done
}

func (f *fakeSubscriber) SetUp(ctx context.Context) {
	f.setupCount++
}

func (f *fakeSubscriber) TearDown(ctx context.Context) {
	f.teardownCount++
}
