package kubernetesdiscovery

import "github.com/tilt-dev/tilt/internal/store"

type Dispatcher interface {
	Dispatch(action store.Action)
}
