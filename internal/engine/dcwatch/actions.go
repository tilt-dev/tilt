package dcwatch

import (
	"time"

	"github.com/docker/docker/api/types"

	"github.com/windmilleng/tilt/internal/dockercompose"
)

type EventAction struct {
	Event          dockercompose.Event
	Time           time.Time
	ContainerState types.ContainerState
}

func (EventAction) Action() {}
