package engine

import (
	"context"
	"os"
	"os/exec"
	"time"

	"github.com/windmilleng/wmclient/pkg/dirs"

	"github.com/windmilleng/tilt/internal/build"
	"github.com/windmilleng/tilt/internal/engine/configs"
	"github.com/windmilleng/tilt/internal/store"
	"github.com/windmilleng/tilt/internal/tracer"
	"github.com/windmilleng/tilt/pkg/model"
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
		logError(st, err)
		return
	}
	defer func() {
		err := file.Close()
		if err != nil {
			logError(st, err)
		}
	}()

	cmd.Stdin = file

	err = cmd.Run()
	if err != nil {
		logError(st, err)
	}
	cmdSucceeded := err == nil

	st.Dispatch(TelemetryScriptRanAction{At: t.clock.Now()})

	// clear the file if the telemetry command succeeded
	if cmdSucceeded {
		if err = t.dir.WriteFile(tracer.OutgoingFilename, ""); err != nil {
			logError(st, err)
		}
	}
}

func logError(st store.RStore, err error) {
	st.Dispatch(configs.TiltfileLogAction{
		LogEvent: store.NewLogEvent(model.TiltfileManifestName, []byte(err.Error())),
	})
}
