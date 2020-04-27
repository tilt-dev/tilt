package dcwatch

import (
	"time"

	"github.com/windmilleng/tilt/internal/dockercompose"
)

type EventAction struct {
	Event dockercompose.Event
	Time  time.Time
}

func (EventAction) Action() {}
