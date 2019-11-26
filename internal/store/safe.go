package store

import (
	"fmt"
	"runtime/debug"
)

// Execute the function from a goroutine, recovering any panics and exiting
// the program gracefully.
//
// This helps tcell reset the terminal to a state where we can
// print the error correctly, see
// https://github.com/gdamore/tcell/issues/147
// for more discussion.
//
// Otherwise, a panic() would just blow up the terminal and
// force a reset.
func SafeGo(store RStore, f func()) {
	go func() {
		defer func() {
			r := recover()
			if r != nil {
				err := fmt.Errorf("PANIC: %v\n%s", r, debug.Stack())
				store.Dispatch(PanicAction{Err: err})
			}
		}()

		f()
	}()
}
