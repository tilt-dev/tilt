package client

import (
	"fmt"

	"k8s.io/client-go/tools/clientcmd"

	"github.com/tilt-dev/tilt/pkg/model"
)

type TiltClientConfig clientcmd.ClientConfig

// Uses the kubernetes config-loading library to create a client config
// for the given server name.
func ProvideClientConfig(apiServerName model.APIServerName, configAccess clientcmd.ConfigAccess) (TiltClientConfig, error) {
	config, err := configAccess.GetStartingConfig()
	if err != nil {
		return nil, err
	}

	name := string(apiServerName)

	if _, ok := config.Contexts[name]; !ok {
		return nil, fmt.Errorf("No tilt apiserver found: %s", name)
	}

	newCfg := config.DeepCopy()
	newCfg.CurrentContext = name
	return TiltClientConfig(clientcmd.NewDefaultClientConfig(*newCfg, nil)), nil
}
