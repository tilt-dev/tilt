package k8s

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"k8s.io/apimachinery/pkg/util/httpstream"
	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp" // registers gcp auth provider
	"k8s.io/client-go/transport/spdy"

	"github.com/tilt-dev/tilt/internal/k8s/portforward"
	"github.com/tilt-dev/tilt/pkg/logger"

	"github.com/pkg/errors"
)

type PortForwardClient interface {
	// Creates a new port-forwarder that's bound to the given context's lifecycle.
	// When the context is canceled, the port-forwarder will close.
	CreatePortForwarder(ctx context.Context, namespace Namespace, podID PodID, localPort int, remotePort int, host string) (PortForwarder, error)
}

type PortForwarder interface {
	// The local port we're listening on.
	LocalPort() int

	// Addresses that we're listening on.
	Addresses() []string

	// ReadyCh will be closed by ForwardPorts once the forwarding is successfully set up.
	//
	// ForwardPorts might return an error during initialization before forwarding is successfully set up, in which
	// case this channel will NOT be closed.
	ReadyCh() <-chan struct{}

	// Listens on the configured port and forward all traffic to the container.
	// Returns when the port-forwarder sees an unrecoverable error or
	// when the context passed at creation is canceled.
	ForwardPorts() error

	// TODO(nick): If the port forwarder has any problems connecting to the pod,
	// it just logs those as debug logs. I'm not sure that logs are the right API
	// for this -- there are lots of cases (e.g., where you're deliberately
	// restarting the pod) where it's ok if it drops the connection.
	//
	// I suspect what we actually need is a healthcheck/status field for the
	// portforwarder that's exposed as part of the engine.
}

type portForwarder struct {
	*portforward.PortForwarder
	localPort int
}

var _ PortForwarder = portForwarder{}

func (pf portForwarder) LocalPort() int {
	return pf.localPort
}

func (pf portForwarder) ReadyCh() <-chan struct{} {
	return pf.Ready
}

func (k *K8sClient) CreatePortForwarder(ctx context.Context, namespace Namespace, podID PodID, optionalLocalPort, remotePort int, host string) (PortForwarder, error) {
	localPort := optionalLocalPort
	if localPort == 0 {
		// preferably, we'd set the localport to 0, and let the underlying function pick a port for us,
		// to avoid the race condition potential of something else grabbing this port between
		// the call to `getAvailablePort` and whenever `portForwarder` actually binds the port.
		// the k8s client supports a local port of 0, and stores the actual local port assigned in a field,
		// but unfortunately does not export that field, so there is no way for the caller to know which
		// local port to talk to.
		var err error
		localPort, err = getAvailablePort()
		if err != nil {
			return nil, errors.Wrap(err, "failed to find an available local port")
		}
	}

	return k.portForwardClient.CreatePortForwarder(ctx, namespace, podID, localPort, remotePort, host)
}

type newPodDialerFn func(namespace Namespace, podID PodID) (httpstream.Dialer, error)

type portForwardClient struct {
	newPodDialer newPodDialerFn
}

func ProvidePortForwardClient(
	maybeRESTConfig RESTConfigOrError,
	maybeClientset ClientsetOrError) PortForwardClient {
	if maybeRESTConfig.Error != nil {
		return explodingPortForwardClient{error: maybeRESTConfig.Error}
	}
	if maybeClientset.Error != nil {
		return explodingPortForwardClient{error: maybeClientset.Error}
	}

	config := maybeRESTConfig.Config
	core := maybeClientset.Clientset.CoreV1()
	newPodDialer := newPodDialerFn(func(namespace Namespace, podID PodID) (httpstream.Dialer, error) {
		transport, upgrader, err := spdy.RoundTripperFor(config)
		if err != nil {
			return nil, errors.Wrap(err, "error getting roundtripper")
		}

		req := core.RESTClient().Post().
			Resource("pods").
			Namespace(namespace.String()).
			Name(podID.String()).
			SubResource("portforward")

		dialer := spdy.NewDialer(upgrader, &http.Client{Transport: transport}, "POST", req.URL())
		return dialer, nil
	})

	return portForwardClient{
		newPodDialer: newPodDialer,
	}
}

func (c portForwardClient) CreatePortForwarder(ctx context.Context, namespace Namespace, podID PodID, localPort int, remotePort int, host string) (PortForwarder, error) {
	dialer, err := c.newPodDialer(namespace, podID)
	if err != nil {
		return nil, err
	}
	readyChan := make(chan struct{}, 1)

	ports := []string{fmt.Sprintf("%d:%d", localPort, remotePort)}

	var pf *portforward.PortForwarder

	// The tiltfile evaluator always defaults the empty string.
	//
	// If it's defaulting to localhost, use the default kubernetse logic
	// for binding the portforward.
	w := logger.NewMutexWriter(logger.Get(ctx).Writer(logger.DebugLvl))
	if host == "" || host == "localhost" {
		pf, err = portforward.New(
			dialer,
			ports,
			ctx.Done(),
			readyChan, w, w)
	} else {
		var addresses []string
		addresses, err = getListenableAddresses(host)
		if err != nil {
			return nil, err
		}
		pf, err = portforward.NewOnAddresses(
			dialer,
			addresses,
			ports,
			ctx.Done(),
			readyChan, w, w)
	}
	if err != nil {
		return nil, errors.Wrap(err, "error forwarding port")
	}

	return portForwarder{
		PortForwarder: pf,
		localPort:     localPort,
	}, nil
}

func getListenableAddresses(host string) ([]string, error) {
	// handle IPv6 literals like `[::1]`
	url, err := url.Parse(fmt.Sprintf("http://%s/", host))
	if err != nil {
		return nil, errors.Wrap(err, fmt.Sprintf("invalid host %s", host))
	}
	addresses, err := net.LookupHost(url.Hostname())
	if err != nil {
		return nil, errors.Wrap(err, fmt.Sprintf("failed to look up address for %s", host))
	}
	listenable := make([]string, 0)
	for _, addr := range addresses {
		var l net.Listener
		if ipv6 := strings.Contains(addr, ":"); ipv6 {
			// skip ipv6 addresses that include a zone index
			// see: https://github.com/tilt-dev/tilt/issues/5981
			if hasZoneIndex := strings.Contains(addr, "%"); hasZoneIndex {
				continue
			}

			l, err = net.Listen("tcp6", fmt.Sprintf("[%s]:0", addr))
		} else {
			l, err = net.Listen("tcp4", fmt.Sprintf("%s:0", addr))
		}
		if err == nil {
			l.Close()
			listenable = append(listenable, addr)
		}
	}
	if len(listenable) == 0 {
		return nil, errors.Errorf("host %s: cannot listen on any resolved addresses: %v", host, addresses)
	}
	return listenable, nil
}

func getAvailablePort() (int, error) {
	l, err := net.Listen("tcp", ":0")
	if err != nil {
		return 0, err
	}
	defer func() {
		e := l.Close()
		if err == nil {
			err = e
		}
	}()

	_, p, err := net.SplitHostPort(l.Addr().String())
	if err != nil {
		return 0, err
	}
	port, err := strconv.Atoi(p)
	if err != nil {
		return 0, err
	}
	return port, err
}

type explodingPortForwardClient struct {
	error error
}

func (c explodingPortForwardClient) CreatePortForwarder(ctx context.Context, namespace Namespace, podID PodID, localPort int, remotePort int, host string) (PortForwarder, error) {
	return nil, c.error
}
