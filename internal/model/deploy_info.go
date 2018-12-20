package model

import (
	"reflect"

	"github.com/windmilleng/tilt/internal/yaml"
)

type deployInfo interface {
	deployInfo()
}

type DCInfo struct {
	ConfigPath string
	YAMLRaw    []byte // for diff'ing when config files change
	DfRaw      []byte // for diff'ing when config files change
}

func (DCInfo) deployInfo()    {}
func (dc DCInfo) Empty() bool { return reflect.DeepEqual(dc, DCInfo{}) }

type K8sInfo struct {
	YAML         string
	portForwards []PortForward
}

func (K8sInfo) deployInfo()     {}
func (k8s K8sInfo) Empty() bool { return reflect.DeepEqual(k8s, K8sInfo{}) }

func (k8s K8sInfo) AppendYAML(y string) K8sInfo {
	if k8s.YAML == "" {
		k8s.YAML = y
	} else {
		k8s.YAML = yaml.ConcatYAML(k8s.YAML, y)
	}
	return k8s
}
