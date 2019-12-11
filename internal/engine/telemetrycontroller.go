package engine

import (
	"context"
	"os"
	"os/exec"
	"time"

	"github.com/windmilleng/wmclient/pkg/dirs"

	"github.com/windmilleng/tilt/internal/build"
	"github.com/windmilleng/tilt/internal/store"
	"github.com/windmilleng/tilt/internal/tracer"
)

type TelemetryController struct {
	clock build.Clock
	// lock is a lock that is used to control access from the span exporter and the telemetry controller to the file
	lock tracer.Locker
	dir  *dirs.WindmillDir
}

func NewTelemetryController(lock tracer.Locker, clock build.Clock, dir *dirs.WindmillDir) *TelemetryController {
	return &TelemetryController{
		lock:  lock,
		clock: clock,
		dir:   dir,
	}
}

// TODO(dmiller): make these errors log actions
func (t *TelemetryController) OnChange(ctx context.Context, st store.RStore) {
	state := st.RLockState()
	lastTelemetryRun := state.LastTelemetryScriptRun
	tc := state.TelemetryCmd
	st.RUnlockState()

	if tc.Empty() || !lastTelemetryRun.Add(1*time.Hour).Before(t.clock.Now()) {
		return
	}

	t.lock.Lock()
	defer t.lock.Unlock()

	// exec the telemetry command, passing in the contents of the file on stdin
	cmd := exec.CommandContext(ctx, tc.Argv[0], tc.Argv[1:]...)
	file, err := t.dir.OpenFile(tracer.OutgoingFilename, os.O_RDONLY, 0644)
	if err != nil {
		st.Dispatch(NewErrorAction(err))
		return
	}
	defer file.Close()

	cmd.Stdin = file

	err = cmd.Run()
	if err != nil {
		st.Dispatch(NewErrorAction(err))
	}
	cmdSucceeded := err == nil

	st.Dispatch(TelemetryScriptRanAction{At: t.clock.Now()})

	// truncate the file if the telemetry command succeeded
	if cmdSucceeded {
		if err = t.dir.WriteFile(tracer.OutgoingFilename, ""); err != nil {
			st.Dispatch(NewErrorAction(err))
		}
	}
}
