package apiserver

import (
	"context"
	"net"
)

type ConnProvider interface {
	Dial(network, address string) (net.Conn, error)
	DialContext(ctx context.Context, network, address string) (net.Conn, error)
	Listen(network, address string) (net.Listener, error)
}

// Creates a ConnProvider where all connections are redirected over a particular
// network. Useful for use with memconn, which uses "memb" and "memu"
// as in-memory networks.
func NetworkConnProvider(p ConnProvider, network string) ConnProvider {
	return networkConnProvider{
		delegate: p,
		network:  network,
	}
}

type networkConnProvider struct {
	delegate ConnProvider
	network  string
}

func (p networkConnProvider) Dial(network, address string) (net.Conn, error) {
	return p.delegate.Dial(p.network, address)
}
func (p networkConnProvider) DialContext(ctx context.Context, network, address string) (net.Conn, error) {
	return p.delegate.DialContext(ctx, p.network, address)
}
func (p networkConnProvider) Listen(network, address string) (net.Listener, error) {
	return p.delegate.Listen(p.network, address)
}
