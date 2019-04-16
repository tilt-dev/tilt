package network

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestIsLocalhostFree(t *testing.T) {
	// Find a free port
	port := 0
	for port = 10000; port < 10100; port++ {
		if IsBindAddrFree(LocalhostBindAddr(port)) != nil {
			break
		}
	}

	// bind that port on localhost
	l, err := bindAddress(LocalhostBindAddr(port))
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()

	// assert that the port is now bound
	err = IsBindAddrFree(LocalhostBindAddr(port))
	if assert.Error(t, err) {
		assert.Contains(t, err.Error(), "bind")
	}
}
