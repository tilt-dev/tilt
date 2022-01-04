package cluster

import (
	"testing"

	"github.com/stretchr/testify/require"
	"k8s.io/client-go/rest"
)

func TestHashRESTConfig(t *testing.T) {
	a := hashClientConfig(&rest.Config{Host: "example.com"}, "foo")
	require.NotZero(t, a, "Config hash should not be empty")
	b := hashClientConfig(&rest.Config{Host: "example.com", QPS: 1.0}, "foo")
	require.Equal(t, a, b, "Config hashes were not equal")

	b = hashClientConfig(&rest.Config{Host: "other.example.com"}, "foo")
	require.NotZero(t, b, "Config hash should not be empty")
	require.NotEqualf(t, a, b, "Client hashes should not be equal")

	b = hashClientConfig(&rest.Config{Host: "example.com"}, "bar")
	require.NotZero(t, b, "Config hash should not be empty")
	require.NotEqualf(t, a, b, "Client hashes should not be equal")
}
