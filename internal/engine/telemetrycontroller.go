package engine

import (
	"context"
	"fmt"
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
	lock   tracer.Locker
	dir    *dirs.WindmillDir
	execer func(ctx context.Context, name string, arg ...string) *exec.Cmd
}

func NewTelemetryController(lock tracer.Locker, clock build.Clock, dir *dirs.WindmillDir, execer func(ctx context.Context, name string, arg ...string) *exec.Cmd) *TelemetryController {
	return &TelemetryController{
		lock:   lock,
		clock:  clock,
		dir:    dir,
		execer: execer,
	}
}

// TODO(dmiller): make these errors log actions
func (t *TelemetryController) OnChange(ctx context.Context, st store.RStore) {
	state := st.RLockState()
	lastTelemetryRun := state.LastTelemetryScriptRun
	tc := state.TelemetryCmd
	st.RUnlockState()

	if lastTelemetryRun.Add(1*time.Hour).Before(t.clock.Now()) && !tc.Empty() {
		t.lock.Lock()
		defer t.lock.Unlock()

		// exec the telemetry command, passing in the contents of the file on stdin
		cmd := t.execer(ctx, tc.String())
		stdin, err := cmd.StdinPipe()
		if err != nil {
			st.Dispatch(NewErrorAction(err))
			return
		}
		if stdin == nil {
			st.Dispatch(NewErrorAction(fmt.Errorf("Unable to open stdin to telemetry command")))
			return
		}
		defer stdin.Close()

		file, err := t.dir.OpenFile(tracer.OutgoingFilename, os.O_RDONLY, 0644)
		if err != nil {
			st.Dispatch(NewErrorAction(err))
			return
		}

		cmd.Stdin = file

		err = cmd.Run()
		if err != nil {
			st.Dispatch(NewErrorAction(err))
		}
		cmdSucceeded := err == nil

		st.Dispatch(TelemetryScriptRanAction{At: t.clock.Now()})

		// truncate the file if the telemetry command succeeded
		if cmdSucceeded {
			err = t.clearAnalyticsFile()
			if err != nil {
				st.Dispatch(NewErrorAction(err))
			}
		}
	}
}

func (t *TelemetryController) clearAnalyticsFile() error {
	return t.dir.WriteFile(tracer.OutgoingFilename, "")
}

func ProvideExecer() func(ctx context.Context, name string, arg ...string) *exec.Cmd {
	return exec.CommandContext
}
