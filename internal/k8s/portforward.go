package k8s

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"strconv"

	v1 "k8s.io/client-go/kubernetes/typed/core/v1"
	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp" // registers gcp auth provider
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/portforward"
	"k8s.io/client-go/transport/spdy"

	"github.com/windmilleng/tilt/pkg/logger"

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

	// Listens on the configured port and forward all traffic to the container.
	// Returns when the port-forwarder sees an unrecoverable error or
	// when the context passed at creation is canceled.
	ForwardPorts() error
}

type portForwarder struct {
	*portforward.PortForwarder
	localPort int
}

func (pf portForwarder) LocalPort() int {
	return pf.localPort
}

func (k K8sClient) CreatePortForwarder(ctx context.Context, namespace Namespace, podID PodID, optionalLocalPort, remotePort int, host string) (PortForwarder, error) {
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

type portForwardClient struct {
	config *rest.Config
	core   v1.CoreV1Interface
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
	return portForwardClient{
		maybeRESTConfig.Config,
		maybeClientset.Clientset.CoreV1(),
	}
}

func (c portForwardClient) CreatePortForwarder(ctx context.Context, namespace Namespace, podID PodID, localPort int, remotePort int, host string) (PortForwarder, error) {
	transport, upgrader, err := spdy.RoundTripperFor(c.config)
	if err != nil {
		return nil, errors.Wrap(err, "error getting roundtripper")
	}

	req := c.core.RESTClient().Post().
		Resource("pods").
		Namespace(namespace.String()).
		Name(podID.String()).
		SubResource("portforward")

	dialer := spdy.NewDialer(upgrader, &http.Client{Transport: transport}, "POST", req.URL())
	if err != nil {
		return nil, errors.Wrap(err, "error creating dialer")
	}

	stopChan := make(chan struct{}, 1)
	readyChan := make(chan struct{}, 1)

	ports := []string{fmt.Sprintf("%d:%d", localPort, remotePort)}

	var pf *portforward.PortForwarder
	if host == "" {
		pf, err = portforward.New(
			dialer,
			ports,
			stopChan,
			readyChan,
			logger.Get(ctx).Writer(logger.DebugLvl),
			logger.Get(ctx).Writer(logger.DebugLvl))
	} else {
		addresses := []string{host}
		pf, err = portforward.NewOnAddresses(
			dialer,
			addresses,
			ports,
			stopChan,
			readyChan,
			logger.Get(ctx).Writer(logger.DebugLvl),
			logger.Get(ctx).Writer(logger.DebugLvl))
	}
	if err != nil {
		return nil, errors.Wrap(err, "error forwarding port")
	}

	go func() {
		<-ctx.Done()
		close(stopChan)
	}()
	return portForwarder{
		PortForwarder: pf,
		localPort:     localPort,
	}, nil
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
