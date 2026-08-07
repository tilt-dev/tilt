package server

import (
	"fmt"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	genericapiserver "k8s.io/apiserver/pkg/server"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"

	"github.com/tilt-dev/wmclient/pkg/dirs"

	"github.com/tilt-dev/tilt/internal/filelock"
	"github.com/tilt-dev/tilt/internal/testutils/tempdir"
	"github.com/tilt-dev/tilt/pkg/model"
)

// Concurrent Tilt processes share the same config file
// (~/.tilt-dev/config). Registration and removal must be atomic
// read-modify-write operations; otherwise one process can write a stale
// snapshot that discards another process's entry, and clients later fail
// with "No tilt apiserver found".
func TestConcurrentAPIServerConfigUpdates(t *testing.T) {
	f := tempdir.NewTempDirFixture(t)
	configAccess := ProvideConfigAccess(dirs.NewTiltDevDirAt(f.Path()))

	const count = 8
	controllers := make([]*HeadsUpServerController, count)
	for i := range controllers {
		controllers[i] = newConfigOnlyController(
			configAccess, model.APIServerName(fmt.Sprintf("tilt-%d", 10000+i)))
	}

	var wg sync.WaitGroup
	for _, c := range controllers {
		wg.Add(1)
		go func(c *HeadsUpServerController) {
			defer wg.Done()
			assert.NoError(t, c.addToAPIServerConfig())
		}(c)
	}
	wg.Wait()

	config := readConfigForTest(t, configAccess)
	for _, c := range controllers {
		name := string(c.apiServerName)
		if assert.Contains(t, config.Contexts, name) {
			assert.Contains(t, config.AuthInfos, name)
			assert.Contains(t, config.Clusters, name)
		}
	}

	for _, c := range controllers {
		wg.Add(1)
		go func(c *HeadsUpServerController) {
			defer wg.Done()
			assert.NoError(t, c.removeFromAPIServerConfig())
		}(c)
	}
	wg.Wait()

	config = readConfigForTest(t, configAccess)
	assert.Empty(t, config.Contexts)
	assert.Empty(t, config.AuthInfos)
	assert.Empty(t, config.Clusters)
}

// newConfigOnlyController builds the minimal controller needed to
// exercise config registration, without starting any servers.
func newConfigOnlyController(configAccess clientcmd.ConfigAccess, name model.APIServerName) *HeadsUpServerController {
	apiServerConfig := &APIServerConfig{
		GenericConfig: &genericapiserver.RecommendedConfig{
			Config: genericapiserver.Config{
				LoopbackClientConfig: &rest.Config{
					Host:        fmt.Sprintf("http://localhost/%s", name),
					BearerToken: string(name),
				},
			},
		},
	}
	return &HeadsUpServerController{
		configAccess:    configAccess,
		apiServerName:   name,
		apiServerConfig: apiServerConfig,
	}
}

func readConfigForTest(t testing.TB, configAccess clientcmd.ConfigAccess) *clientcmdapi.Config {
	t.Helper()
	var config *clientcmdapi.Config
	err := filelock.WithRLock(configAccess, func() error {
		var e error
		config, e = configAccess.GetStartingConfig()
		return e
	})
	require.NoError(t, err)
	return config
}
