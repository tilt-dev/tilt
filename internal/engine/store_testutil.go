package engine

import (
	"fmt"
	"reflect"
	"testing"
	"time"

	"github.com/windmilleng/tilt/internal/store"
)

// for use by tests, to wait until an action of the specified type comes out of the given chan
func waitForAction(t testing.TB, typ reflect.Type, actions <-chan store.Action) store.Action {
	timeout := time.After(300 * time.Millisecond)
	var seenActions []store.Action
	for {
		select {
		case <-timeout:
			t.Fatalf("timed out waiting for action of type %s. Saw the following actions while waiting: %+v",
				typ.Name(),
				seenActions)
		case a := <-actions:
			if reflect.TypeOf(a) == typ {
				return a
			} else if la, ok := a.(LogAction); ok {
				fmt.Println(string(la.Log))
			} else {
				seenActions = append(seenActions, a)
			}
		}
	}
}
