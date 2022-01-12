package fake

import (
	"io"

	"github.com/tilt-dev/tilt/internal/store"
)

type testStore struct {
	*store.TestingStore
	out io.Writer
}

func NewTestingStore(out io.Writer) *testStore {
	return &testStore{
		TestingStore: store.NewTestingStore(),
		out:          out,
	}
}

func (s *testStore) Dispatch(action store.Action) {
	s.TestingStore.Dispatch(action)
	switch action := action.(type) {
	case store.LogAction:
		_, _ = s.out.Write(action.Message())
	}
}
