package k8s

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"strconv"

	"k8s.io/client-go/kubernetes/typed/core/v1"
	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp" // registers gcp auth provider
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/portforward"
	"k8s.io/client-go/transport/spdy"

	"github.com/windmilleng/tilt/internal/logger"

	"github.com/pkg/errors"
)

func (k K8sClient) ForwardPort(ctx context.Context, namespace string, podID PodID, remotePort int) (localPort int, closer func(), err error) {
	// preferably, we'd set the localport to 0, and let the underlying function pick a port for us,
	// to avoid the race condition potential of something else grabbing this port between
	// the call to `getAvailablePort` and whenever `portForwarder` actually binds the port.
	// the k8s client supports a local port of 0, and stores the actual local port assigned in a field,
	// but unfortunately does not export that field, so there is no way for the caller to know which
	// local port to talk to.
	localPort, err = getAvailablePort()
	if err != nil {
		return 0, nil, errors.Wrap(err, "failed to find an available local port")
	}

	closer, err = k.portForwarder(ctx, k.restConfig, k.core, namespace, podID, localPort, remotePort)
	if err != nil {
		return 0, nil, err
	}

	return localPort, closer, nil
}

func portForwarder(ctx context.Context, restConfig *rest.Config, core v1.CoreV1Interface, namespace string, podID PodID, localPort int, remotePort int) (closer func(), err error) {
	transport, upgrader, err := spdy.RoundTripperFor(restConfig)
	if err != nil {
		return nil, errors.Wrap(err, "error getting roundtripper")
	}

	req := core.RESTClient().Post().
		Resource("pods").
		Namespace(namespace).
		Name(podID.String()).
		SubResource("portforward")

	dialer := spdy.NewDialer(upgrader, &http.Client{Transport: transport}, "POST", req.URL())
	if err != nil {
		return nil, errors.Wrap(err, "error creating dialer")
	}

	stopChan := make(chan struct{}, 1)
	readyChan := make(chan struct{}, 1)

	ports := []string{fmt.Sprintf("%d:%d", localPort, remotePort)}
	pf, err := portforward.New(
		dialer,
		ports,
		stopChan,
		readyChan,
		logger.Get(ctx).Writer(logger.DebugLvl),
		logger.Get(ctx).Writer(logger.InfoLvl))

	if err != nil {
		return nil, errors.Wrap(err, "error forwarding port")
	}

	errChan := make(chan error)
	go func() {
		errChan <- pf.ForwardPorts()
		err := <-errChan
		// logging isn't really sufficient, since we're in a goroutine and who knows where the caller
		// has moved on to by this point, but other options are much more expensive (e.g., monitoring the state
		// of the port forward from the caller and/or automatically reconnecting port forwards)
		logger.Get(ctx).Infof("error from port forward: %v", err)
	}()

	select {
	case err = <-errChan:
		return nil, errors.Wrap(err, "error forwarding port")
	case <-pf.Ready:
		closer = func() {
			close(stopChan)
		}
		return closer, nil
	}
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
