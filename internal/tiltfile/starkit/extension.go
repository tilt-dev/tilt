package starkit

import "go.starlark.net/starlark"

// An extension to a starlark execution environment.
type Extension interface {
	// Called when execution begins.
	OnStart(e *Environment) error
}

type OnExecExtension interface {
	Extension

	// Called before each new Starlark file is loaded
	OnExec(t *starlark.Thread, path string) error
}

type OnBuiltinCallExtension interface {
	Extension

	// Called before each builtin is called
	OnBuiltinCall(name string, fn *starlark.Builtin)
}

// Starkit extensions are not allowed to have mutable state.
//
// Starlark has different ideas about mutable state than most programming languages.
// In particular, state is mutable during file execution, but becomes immutable
// when it's imported into other files.
//
// This allows Starlark to execute files in parallel without locks.
//
// To play more nicely with Starlark, Starkit extensions manage state
// with an init/reduce pattern. That means each extension should define:
//
// 1) An initial state (created with NewModel())
// 2) Make subsequent mutations to state with starkit.SetState
//
// At the end of execution, Starkit will return a data model
// with the accumulated state from all extensions.
//
// See: https://github.com/google/starlark-go/blob/master/doc/spec.md#identity-and-mutation
type StatefulExtension interface {
	Extension

	NewState() interface{}
}
