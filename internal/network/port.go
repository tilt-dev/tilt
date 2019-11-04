package network

import (
	"fmt"
	"net"
)

// An address spec for listening on a specified host and port
func BindAddr(host string, port int) string {
	return fmt.Sprintf("%s:%d", host, port)
}

// An address spec for listening on a port on 0.0.0.0.
func AllHostsBindAddr(port int) string {
	return fmt.Sprintf(":%d", port)
}

// Checks if no one is listening on the current address.
func IsBindAddrFree(addr string) error {
	l, err := bindAddress(addr)
	if err != nil {
		return err
	}
	_ = l.Close()

	return err
}

func bindAddress(address string) (net.Listener, error) {
	addr, err := net.ResolveTCPAddr("tcp", address)
	if err != nil {
		return nil, err
	}

	return net.ListenTCP("tcp", addr)
}
