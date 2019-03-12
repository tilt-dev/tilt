package engine

import (
	"fmt"
	"reflect"
	"testing"
	"time"

	"github.com/windmilleng/tilt/internal/store"
)

// for use by tests, to wait until an action of the specified type comes out of the given chan
// at some point we might want it to return the index it was found at, and then take an index,
// so that we can start searching from the next index
func waitForAction(t testing.TB, typ reflect.Type, getActions func() []store.Action) store.Action {
	start := time.Now()
	timeout := 300 * time.Millisecond

	for time.Since(start) < timeout {
		actions := getActions()
		for _, a := range actions {
			if reflect.TypeOf(a) == typ {
				return a
			} else if la, ok := a.(LogAction); ok {
				fmt.Println(string(la.logEvent.Message()))
			}
		}
	}
	t.Fatalf("timed out waiting for action of type %s. Saw the following actions while waiting: %+v",
		typ.Name(),
		getActions())
	return nil
}
