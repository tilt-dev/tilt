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
	context            k8s.KubeContext
	env                k8s.Env
	allowedK8sContexts []k8s.KubeContext
}

func NewExtension(context k8s.KubeContext, env k8s.Env) *Extension {
	return &Extension{
		context: context,
		env:     env,
	}
}

func (e *Extension) OnStart(env *starkit.Environment) error {
	e.allowedK8sContexts = nil

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

func (e *Extension) k8sContext(thread *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	return starlark.String(e.context), nil
}

func (e *Extension) KubeContext() k8s.KubeContext {
	return e.context
}

func (e *Extension) IsAllowed() bool {
	if e.env == k8s.EnvNone || e.env.IsLocalCluster() {
		return true
	}

	for _, c := range e.allowedK8sContexts {
		if c == e.context {
			return true
		}
	}

	return false
}

func (e *Extension) allowK8sContexts(thread *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var contexts starlark.Value
	if err := starkit.UnpackArgs(thread, fn.Name(), args, kwargs,
		"contexts", &contexts,
	); err != nil {
		return nil, err
	}

	for _, c := range value.ValueOrSequenceToSlice(contexts) {
		switch val := c.(type) {
		case starlark.String:
			e.allowedK8sContexts = append(e.allowedK8sContexts, k8s.KubeContext(val))
		default:
			return nil, fmt.Errorf("allow_k8s_contexts contexts must be a string or a sequence of strings; found a %T", val)

		}
	}

	return starlark.None, nil
}
