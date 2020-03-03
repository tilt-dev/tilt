package container

import "strings"

// A good way to manually test different container runtimes is with minikube.
// https://github.com/kubernetes/minikube/blob/master/docs/alternative_runtimes.md
type Runtime string

const (
	RuntimeDocker      Runtime = "docker"
	RuntimeContainerd  Runtime = "containerd"
	RuntimeCrio        Runtime = "cri-o"
	RuntimeUnknown     Runtime = "unknown"
	RuntimeReadFailure Runtime = "read-failure"
)

func RuntimeFromVersionString(s string) Runtime {
	parts := strings.Split(s, ":")
	switch Runtime(parts[0]) {
	case RuntimeDocker:
		return RuntimeDocker
	case RuntimeContainerd:
		return RuntimeContainerd
	case RuntimeCrio:
		return RuntimeCrio
	}
	return RuntimeUnknown
}
