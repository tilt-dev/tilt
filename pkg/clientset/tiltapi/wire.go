package tiltapi

import (
	"fmt"

	"k8s.io/client-go/rest"

	"github.com/tilt-dev/tilt/pkg/model"
)

func ProvideRESTConfig(apiserverHost model.WebHost, apiserverPort model.WebPort) *rest.Config {
	return &rest.Config{
		Host: fmt.Sprintf("http://%s:%d", string(apiserverHost), int(apiserverPort)),
	}
}

func ProvideClientSet(config *rest.Config) (*Clientset, error) {
	return NewForConfig(config)
}
