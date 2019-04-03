package network

import (
	"fmt"
	"net"
)

// Checks if no one is listening on the current TCP port.
func IsPortFree(port int) bool {
	addr, err := net.ResolveTCPAddr("tcp", fmt.Sprintf("localhost:%d", port))
	if err != nil {
		return false
	}

	l, err := net.ListenTCP("tcp", addr)
	if err != nil {
		return false
	}
	_ = l.Close()

	return true
}
