package k8scontext

import (
	"fmt"

	"go.starlark.net/starlark"

	"github.com/tilt-dev/clusterid"
	"github.com/tilt-dev/tilt/internal/k8s"
	"github.com/tilt-dev/tilt/internal/tiltfile/starkit"
	"github.com/tilt-dev/tilt/internal/tiltfile/value"
	"github.com/tilt-dev/tilt/pkg/apis/core/v1alpha1"
	"github.com/tilt-dev/tilt/pkg/model"
)

// Implements functions for dealing with the Kubernetes context.
// Exposes an API for other plugins to get and validate the allowed k8s context.
type Plugin struct {
	context k8s.KubeContext
	env     clusterid.Product
}

func NewPlugin(context k8s.KubeContext, env clusterid.Product) Plugin {
	return Plugin{
		context: context,
		env:     env,
	}
}

func (e Plugin) NewState() interface{} {
	return State{context: e.context, env: e.env}
}

func (e Plugin) OnStart(env *starkit.Environment) error {
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

func (e Plugin) k8sContext(thread *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	return starlark.String(e.context), nil
}

func (e Plugin) allowK8sContexts(thread *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
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

var _ starkit.StatefulPlugin = &Plugin{}

type State struct {
	context k8s.KubeContext
	env     clusterid.Product
	allowed []k8s.KubeContext
}

func (s State) KubeContext() k8s.KubeContext {
	return s.context
}

// Returns whether we're allowed to deploy to this kubecontext.
//
// Checks against a manually specified list and a baked-in list
// with known dev cluster names.
//
// Currently, only the tiltfile executor knows about "allowed" kubecontexts.
//
// We don't keep this information around after tiltfile execution finishes.
//
// This is incompatible with the overall technical direction of tilt as an
// apiserver.  Objects registered via the API (like KubernetesApplys) don't get
// this protection. And it's currently only limited to the main Tiltfile.
//
// A more compatible solution would be to have api server objects
// for the kubecontexts that tilt is aware of, and ways to mark them safe.
func (s State) IsAllowed(tf *v1alpha1.Tiltfile) bool {
	if tf.Name != model.MainTiltfileManifestName.String() {
		return true
	}

	if s.env == k8s.ProductNone || s.env.IsDevCluster() {
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
