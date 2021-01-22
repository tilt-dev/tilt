package tiltfile

import (
	"strings"

	"go.starlark.net/starlark"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/intstr"

	"github.com/tilt-dev/tilt/internal/tiltfile/value"
)

type probe struct {
	spec *v1.Probe
}

func (p probe) String() string {
	return p.spec.String()
}

func (p probe) Type() string {
	return "probe"
}

func (p probe) Freeze() {}

func (p probe) Truth() starlark.Bool {
	return p.spec != nil
}

func (p probe) Hash() (uint32, error) {
	return starlark.String(p.spec.String()).Hash()
}

var _ starlark.Value = probe{}

func (s *tiltfileState) probe(_ *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var initialDelayVal, timeoutVal, periodVal, successThresholdVal, failureThresholdVal value.Int32Value
	var exec execAction
	var httpGet httpGetAction
	var tcpSocket tcpSocketAction
	err := s.unpackArgs(fn.Name(), args, kwargs,
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
		InitialDelaySeconds: int32(initialDelayVal),
		TimeoutSeconds:      int32(timeoutVal),
		PeriodSeconds:       int32(periodVal),
		SuccessThreshold:    int32(successThresholdVal),
		FailureThreshold:    int32(failureThresholdVal),
		Handler: v1.Handler{
			HTTPGet:   httpGet.action,
			Exec:      exec.action,
			TCPSocket: tcpSocket.action,
		},
	}

	return probe{spec: spec}, nil
}

type execAction struct {
	action *v1.ExecAction
}

func (e execAction) String() string {
	return e.action.String()
}

func (e execAction) Type() string {
	return "exec_action"
}

func (e execAction) Freeze() {}

func (e execAction) Truth() starlark.Bool {
	return e.action != nil
}

func (e execAction) Hash() (uint32, error) {
	return starlark.String(e.action.String()).Hash()
}

func (s *tiltfileState) execAction(_ *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var command value.StringSequence
	err := s.unpackArgs(fn.Name(), args, kwargs, "command", &command)
	if err != nil {
		return nil, err
	}
	spec := &v1.ExecAction{Command: []string(command)}
	return execAction{action: spec}, nil
}

type httpGetAction struct {
	action *v1.HTTPGetAction
}

func (h httpGetAction) String() string {
	return h.action.String()
}

func (h httpGetAction) Type() string {
	return "http_get_action"
}

func (h httpGetAction) Freeze() {}

func (h httpGetAction) Truth() starlark.Bool {
	return h.action != nil
}

func (h httpGetAction) Hash() (uint32, error) {
	return starlark.String(h.action.String()).Hash()
}

func (s *tiltfileState) httpGetAction(_ *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var host, scheme, path starlark.String
	var port value.Int32Value
	// TODO(milas): support headers
	err := s.unpackArgs(fn.Name(), args, kwargs,
		"host", &host,
		"port", &port,
		"scheme?", &scheme,
		"path?", &path,
	)
	if err != nil {
		return nil, err
	}

	spec := &v1.HTTPGetAction{
		Host:   host.GoString(),
		Port:   intstr.FromInt(int(port)),
		Scheme: v1.URIScheme(strings.ToUpper(scheme.GoString())),
		Path:   path.GoString(),
	}

	return httpGetAction{action: spec}, nil
}

type tcpSocketAction struct {
	action *v1.TCPSocketAction
}

func (t tcpSocketAction) String() string {
	return t.action.String()
}

func (t tcpSocketAction) Type() string {
	return "tcp_socket_action"
}

func (t tcpSocketAction) Freeze() {}

func (t tcpSocketAction) Truth() starlark.Bool {
	return t.action != nil
}

func (t tcpSocketAction) Hash() (uint32, error) {
	return starlark.String(t.action.String()).Hash()
}

func (s *tiltfileState) tcpSocketAction(_ *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var host starlark.String
	var port value.Int32Value
	err := s.unpackArgs(fn.Name(), args, kwargs, "host", &host, "port", &port)
	if err != nil {
		return nil, err
	}
	spec := &v1.TCPSocketAction{Host: host.GoString(), Port: intstr.FromInt(int(port))}
	return tcpSocketAction{action: spec}, nil
}
