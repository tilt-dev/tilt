package network

import (
	"fmt"
	"net"
)

// Checks if no one is listening on the current TCP port.
func IsPortFree(port int) error {
	addr, err := net.ResolveTCPAddr("tcp", fmt.Sprintf("localhost:%d", port))
	if err != nil {
		return err
	}

	l, err := net.ListenTCP("tcp", addr)
	if err != nil {
		return err
	}
	_ = l.Close()

	return err
}
