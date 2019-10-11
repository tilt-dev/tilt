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
	OnExec(path string)
}

type OnBuiltinCallExtension interface {
	Extension

	// Called before each builtin is called
	OnBuiltinCall(name string, fn *starlark.Builtin)
}
