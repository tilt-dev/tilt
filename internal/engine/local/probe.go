package local

import (
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/pkg/errors"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/intstr"

	"github.com/tilt-dev/probe/pkg/probe"
	"github.com/tilt-dev/probe/pkg/prober"
)

var ErrUnsupportedProbeType = errors.New("unsupported probe type")

func ProvideProberManager() ProberManager {
	return prober.NewManager()
}

type ProberManager interface {
	HTTPGet(u *url.URL, headers http.Header) prober.ProberFunc
	TCPSocket(host string, port int) prober.ProberFunc
	Exec(name string, args ...string) prober.ProberFunc
}

func probeWorkerFromSpec(manager ProberManager, probeSpec *v1.Probe, changedFunc probe.StatusChangedFunc, resultFunc probe.ResultFunc) (*probe.Worker, error) {
	probeFunc, err := proberFromSpec(manager, probeSpec)
	if err != nil {
		return nil, err
	}

	var opts []probe.WorkerOption
	if probeSpec.InitialDelaySeconds >= 0 {
		opts = append(opts, probe.WorkerInitialDelay(time.Duration(probeSpec.InitialDelaySeconds)*time.Second))
	}
	if probeSpec.TimeoutSeconds > 0 {
		opts = append(opts, probe.WorkerTimeout(time.Duration(probeSpec.TimeoutSeconds)*time.Second))
	}
	if probeSpec.PeriodSeconds > 0 {
		opts = append(opts, probe.WorkerPeriod(time.Duration(probeSpec.PeriodSeconds)*time.Second))
	}

	if probeSpec.SuccessThreshold > 0 {
		opts = append(opts, probe.WorkerSuccessThreshold(int(probeSpec.SuccessThreshold)))
	}
	if probeSpec.FailureThreshold > 0 {
		opts = append(opts, probe.WorkerFailureThreshold(int(probeSpec.FailureThreshold)))
	}

	if changedFunc != nil {
		opts = append(opts, probe.WorkerOnStatusChange(changedFunc))
	}
	if resultFunc != nil {
		opts = append(opts, probe.WorkerOnProbeResult(resultFunc))
	}

	w := probe.NewWorker(probeFunc, opts...)
	return w, nil
}

func proberFromSpec(manager ProberManager, probeSpec *v1.Probe) (prober.Prober, error) {
	if probeSpec == nil {
		return nil, nil
	} else if probeSpec.Exec != nil {
		return manager.Exec(probeSpec.Exec.Command[0], probeSpec.Exec.Command[1:]...), nil
	} else if probeSpec.HTTPGet != nil {
		u, err := extractURL(probeSpec.HTTPGet)
		if err != nil {
			return nil, err
		}
		return manager.HTTPGet(u, convertHeaders(probeSpec.HTTPGet.HTTPHeaders)), nil
	} else if probeSpec.TCPSocket != nil {
		port, err := extractPort(probeSpec.TCPSocket.Port)
		if err != nil {
			return nil, err
		}
		host := probeSpec.TCPSocket.Host
		if host == "" {
			// K8s defaults to pod IP; since this is a local resource,
			// localhost is a sane default to somewhat mimic that
			// behavior and reduce the amount of boilerplate to define
			// a probe in most cases
			host = "localhost"
		}
		return manager.TCPSocket(host, port), nil
	}

	return nil, ErrUnsupportedProbeType
}

// extractURL converts a K8s HTTP GET probe spec to a Go URL
// adapted from https://github.com/kubernetes/kubernetes/blob/v1.20.2/pkg/kubelet/prober/prober.go#L163-L186
func extractURL(httpGet *v1.HTTPGetAction) (*url.URL, error) {
	port, err := extractPort(httpGet.Port)
	if err != nil {
		return nil, err
	}
	u, err := url.Parse(httpGet.Path)
	if err != nil {
		return nil, err
	}
	u.Scheme = strings.ToLower(string(httpGet.Scheme))
	if u.Scheme == "" {
		// same default as K8s (plain http)
		u.Scheme = "http"
	}
	host := httpGet.Host
	if host == "" {
		// K8s defaults to pod IP; since this is a local resource,
		// localhost is a sane default to somewhat mimic that
		// behavior and reduce the amount of boilerplate to define
		// a probe in most cases
		host = "localhost"
	}
	u.Host = net.JoinHostPort(host, strconv.Itoa(port))
	return u, nil
}

// extractPort converts a K8s multi-type value to a valid port number or returns an error.
// adapted from https://github.com/kubernetes/kubernetes/blob/v1.20.2/pkg/kubelet/prober/prober.go#L203-L223
// (note: this implementation is substantially simplified from K8s - it does not handle "named" ports as that
// 		  does not apply)
func extractPort(v intstr.IntOrString) (int, error) {
	var port int
	switch v.Type {
	case intstr.Int:
		port = v.IntValue()
	case intstr.String:
		var err error
		port, err = strconv.Atoi(v.StrVal)
		if err != nil {
			return 0, fmt.Errorf("invalid port number: %q", v.StrVal)
		}
	}
	if port <= 0 || port > 65535 {
		return 0, fmt.Errorf("port number out of range: %s", v.String())
	}
	return port, nil
}

// convertHeaders creates a stdlib http.Header map from a collection of HTTP header key-value pairs
// adapted from https://github.com/kubernetes/kubernetes/blob/v1.20.2/pkg/kubelet/prober/prober.go#L146-L154
func convertHeaders(headerList []v1.HTTPHeader) http.Header {
	headers := make(http.Header)
	for _, header := range headerList {
		headers[header.Name] = append(headers[header.Name], header.Value)
	}
	return headers
}
