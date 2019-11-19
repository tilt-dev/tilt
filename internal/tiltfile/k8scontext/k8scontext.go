package k8scontext

import (
	"fmt"

	"go.starlark.net/starlark"

	"github.com/windmilleng/tilt/internal/k8s"
	"github.com/windmilleng/tilt/internal/tiltfile/starkit"
	"github.com/windmilleng/tilt/internal/tiltfile/value"
)

// Implements functions for dealing with the Kubernetes context.
// Exposes an API for other plugins to get and validate the allowed k8s context.
type Extension struct {
	context k8s.KubeContext
	env     k8s.Env
}

func NewExtension(context k8s.KubeContext, env k8s.Env) Extension {
	return Extension{
		context: context,
		env:     env,
	}
}

func (e Extension) NewState() interface{} {
	return State{context: e.context, env: e.env}
}

func (e Extension) OnStart(env *starkit.Environment) error {
	err := env.AddBuiltin("allow_k8s_contexts", e.allowK8sContexts)
	if err != nil {
		return err
	}

	err = env.AddBuiltin("k8s_context", e.k8sContext)
	if err != nil {
		return err
	}
	return nil
}

func (e Extension) k8sContext(thread *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	return starlark.String(e.context), nil
}

func (e Extension) allowK8sContexts(thread *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var contexts starlark.Value
	if err := starkit.UnpackArgs(thread, fn.Name(), args, kwargs,
		"contexts", &contexts,
	); err != nil {
		return nil, err
	}

	newContexts := []k8s.KubeContext{}
	for _, c := range value.ValueOrSequenceToSlice(contexts) {
		switch val := c.(type) {
		case starlark.String:
			newContexts = append(newContexts, k8s.KubeContext(val))
		default:
			return nil, fmt.Errorf("allow_k8s_contexts contexts must be a string or a sequence of strings; found a %T", val)

		}
	}

	err := starkit.SetState(thread, func(existing State) State {
		return State{
			context: existing.context,
			env:     existing.env,
			allowed: append(newContexts, existing.allowed...),
		}
	})

	return starlark.None, err
}

var _ starkit.StatefulExtension = &Extension{}

type State struct {
	context k8s.KubeContext
	env     k8s.Env
	allowed []k8s.KubeContext
}

func (s State) KubeContext() k8s.KubeContext {
	return s.context
}

func (s State) IsAllowed() bool {
	if s.env == k8s.EnvNone || s.env.IsLocalCluster() {
		return true
	}

	for _, c := range s.allowed {
		if c == s.context {
			return true
		}
	}

	return false
}

func MustState(model starkit.Model) State {
	state, err := GetState(model)
	if err != nil {
		panic(err)
	}
	return state
}

func GetState(model starkit.Model) (State, error) {
	var state State
	err := model.Load(&state)
	return state, err
}
