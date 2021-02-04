package probe

import (
	"errors"
	"fmt"
	"strings"

	"go.starlark.net/starlark"
	"go.starlark.net/starlarkstruct"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/intstr"

	"github.com/tilt-dev/tilt/internal/tiltfile/starkit"
	"github.com/tilt-dev/tilt/internal/tiltfile/value"
)

var errInvalidProbeAction = errors.New("exactly one of exec, http_get, or tcp_socket must be specified")

func NewExtension() Extension {
	return Extension{}
}

type Extension struct{}

var _ starkit.Extension = Extension{}

func (e Extension) OnStart(env *starkit.Environment) error {
	if err := env.AddBuiltin("http_get_action", e.httpGetAction); err != nil {
		return fmt.Errorf("could not add http_get_action builtin: %v", err)
	}
	if err := env.AddBuiltin("exec_action", e.execAction); err != nil {
		return fmt.Errorf("could not add exec_action builtin: %v", err)
	}
	if err := env.AddBuiltin("tcp_socket_action", e.tcpSocketAction); err != nil {
		return fmt.Errorf("could not add tcp_socket_action builtin: %v", err)
	}
	if err := env.AddBuiltin("probe", e.probe); err != nil {
		return fmt.Errorf("could not add Probe builtin: %v", err)
	}
	return nil
}

type Probe struct {
	*starlarkstruct.Struct
	spec *v1.Probe
}

var _ starlark.Value = Probe{}

// Spec returns the probe specification in the canonical format. It must not be modified.
func (p Probe) Spec() *v1.Probe {
	return p.spec
}

func (e Extension) probe(thread *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var initialDelayVal, timeoutVal, periodVal, successThresholdVal, failureThresholdVal value.Int32
	var exec ExecAction
	var httpGet HTTPGetAction
	var tcpSocket TCPSocketAction
	err := starkit.UnpackArgs(thread, fn.Name(), args, kwargs,
		"initial_delay_secs?", &initialDelayVal,
		"timeout_secs?", &timeoutVal,
		"period_secs?", &periodVal,
		"success_threshold?", &successThresholdVal,
		"failure_threshold?", &failureThresholdVal,
		"exec?", &exec,
		"http_get?", &httpGet,
		"tcp_socket?", &tcpSocket,
	)
	if err != nil {
		return nil, err
	}

	spec := &v1.Probe{
		InitialDelaySeconds: initialDelayVal.Int32(),
		TimeoutSeconds:      timeoutVal.Int32(),
		PeriodSeconds:       periodVal.Int32(),
		SuccessThreshold:    successThresholdVal.Int32(),
		FailureThreshold:    failureThresholdVal.Int32(),
		Handler: v1.Handler{
			HTTPGet:   httpGet.action,
			Exec:      exec.action,
			TCPSocket: tcpSocket.action,
		},
	}

	if err := validateProbeSpec(spec); err != nil {
		return nil, err
	}

	return Probe{
		Struct: starlarkstruct.FromKeywords(starlark.String("probe"), []starlark.Tuple{
			{starlark.String("initial_delay_secs"), initialDelayVal},
			{starlark.String("timeout_secs"), timeoutVal},
			{starlark.String("period_secs"), periodVal},
			{starlark.String("success_threshold"), successThresholdVal},
			{starlark.String("failure_threshold"), failureThresholdVal},
			{starlark.String("exec"), exec.ValueOrNone()},
			{starlark.String("http_get"), httpGet.ValueOrNone()},
			{starlark.String("tcp_socket"), tcpSocket.ValueOrNone()},
		}),
		spec: spec,
	}, nil
}

func validateProbeSpec(spec *v1.Probe) error {
	actionCount := 0
	if spec.Exec != nil {
		actionCount++
	}
	if spec.HTTPGet != nil {
		actionCount++
	}
	if spec.TCPSocket != nil {
		actionCount++
	}
	if actionCount != 1 {
		return errInvalidProbeAction
	}
	return nil
}

type ExecAction struct {
	*starlarkstruct.Struct
	action *v1.ExecAction
}

var _ starlark.Value = ExecAction{}

func (e ExecAction) ValueOrNone() starlark.Value {
	// starlarkstruct does not handle being nil well, so need to explicitly return a NoneType
	// instead of it when embedding in another value (i.e. within the probe)
	if e.Struct != nil {
		return e
	}
	return starlark.None
}

func (e Extension) execAction(thread *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var command value.StringSequence
	err := starkit.UnpackArgs(thread, fn.Name(), args, kwargs, "command", &command)
	if err != nil {
		return nil, err
	}
	spec := &v1.ExecAction{Command: []string(command)}
	return ExecAction{
		Struct: starlarkstruct.FromKeywords(starlark.String("exec_action"), []starlark.Tuple{
			{starlark.String("command"), command.Sequence()},
		}),
		action: spec,
	}, nil
}

type HTTPGetAction struct {
	*starlarkstruct.Struct
	action *v1.HTTPGetAction
}

var _ starlark.Value = HTTPGetAction{}

func (h HTTPGetAction) ValueOrNone() starlark.Value {
	// starlarkstruct does not handle being nil well, so need to explicitly return a NoneType
	// instead of it when embedding in another value (i.e. within the probe)
	if h.Struct != nil {
		return h
	}
	return starlark.None
}

func (e Extension) httpGetAction(thread *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var host, scheme, path starlark.String
	var port int
	// TODO(milas): support headers
	err := starkit.UnpackArgs(thread, fn.Name(), args, kwargs,
		"port", &port,
		"host?", &host,
		"scheme?", &scheme,
		"path?", &path,
	)
	if err != nil {
		return nil, err
	}

	spec := &v1.HTTPGetAction{
		Host:   host.GoString(),
		Port:   intstr.FromInt(port),
		Scheme: v1.URIScheme(strings.ToUpper(scheme.GoString())),
		Path:   path.GoString(),
	}

	return HTTPGetAction{
		Struct: starlarkstruct.FromKeywords(starlark.String("http_get_action"), []starlark.Tuple{
			{starlark.String("host"), host},
			{starlark.String("port"), starlark.MakeInt(port)},
			{starlark.String("scheme"), scheme},
			{starlark.String("path"), path},
		}),
		action: spec,
	}, nil
}

type TCPSocketAction struct {
	*starlarkstruct.Struct
	action *v1.TCPSocketAction
}

var _ starlark.Value = TCPSocketAction{}

func (t TCPSocketAction) ValueOrNone() starlark.Value {
	// starlarkstruct does not handle being nil well, so need to explicitly return a NoneType
	// instead of it when embedding in another value (i.e. within the probe)
	if t.Struct != nil {
		return t
	}
	return starlark.None
}

func (e Extension) tcpSocketAction(thread *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var host starlark.String
	var port int
	err := starkit.UnpackArgs(thread, fn.Name(), args, kwargs,
		"port", &port,
		"host?", &host,
	)
	if err != nil {
		return nil, err
	}
	spec := &v1.TCPSocketAction{Host: host.GoString(), Port: intstr.FromInt(port)}
	return TCPSocketAction{
		Struct: starlarkstruct.FromKeywords(starlark.String("tcp_socket_action"), []starlark.Tuple{
			{starlark.String("host"), host},
			{starlark.String("port"), starlark.MakeInt(port)},
		}),
		action: spec,
	}, nil
}
