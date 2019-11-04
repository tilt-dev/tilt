package network

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

const Localhost = "localhost"

func TestIsLocalhostFree(t *testing.T) {
	// Find a free port
	port := 0
	for port = 10000; port < 10100; port++ {
		if IsBindAddrFree(BindAddr(Localhost, port)) != nil {
			break
		}
	}

	// bind that port on localhost
	l, err := bindAddress(BindAddr(Localhost, port))
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()

	// assert that the port is now bound
	err = IsBindAddrFree(BindAddr(Localhost, port))
	if assert.Error(t, err) {
		assert.Contains(t, err.Error(), "bind")
	}
}
