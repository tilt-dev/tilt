package telemetry

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"time"

	"github.com/windmilleng/wmclient/pkg/dirs"

	"github.com/windmilleng/tilt/internal/build"
	"github.com/windmilleng/tilt/internal/engine/configs"
	"github.com/windmilleng/tilt/internal/store"
	"github.com/windmilleng/tilt/internal/tracer"
	"github.com/windmilleng/tilt/pkg/model"
	"github.com/windmilleng/tilt/pkg/model/logstore"
)

type Controller struct {
	clock build.Clock
	// lock is a lock that is used to control access from the span exporter and the telemetry controller to the file
	lock       tracer.Locker
	dir        *dirs.WindmillDir
	runCounter int
}

func NewController(lock tracer.Locker, clock build.Clock, dir *dirs.WindmillDir) *Controller {
	return &Controller{
		lock:       lock,
		clock:      clock,
		dir:        dir,
		runCounter: 0,
	}
}

func (t *Controller) OnChange(ctx context.Context, st store.RStore) {
	state := st.RLockState()
	lastTelemetryRun := state.LastTelemetryScriptRun
	tc := state.TelemetryCmd
	st.RUnlockState()

	t.maybeRunScript(ctx, tc, lastTelemetryRun, st)
	t.runCounter++
}

func (t *Controller) maybeRunScript(ctx context.Context, tc model.Cmd, lastTelemetryRun time.Time, st store.RStore) {
	if tc.Empty() || !lastTelemetryRun.Add(1*time.Hour).Before(t.clock.Now()) {
		return
	}

	t.lock.Lock()
	defer t.lock.Unlock()

	// exec the telemetry command, passing in the contents of the file on stdin
	cmd := exec.CommandContext(ctx, tc.Argv[0], tc.Argv[1:]...)
	file, err := t.dir.OpenFile(tracer.OutgoingFilename, os.O_RDONLY, 0644)
	if err != nil {
		t.logError(st, err)
		return
	}
	defer func() {
		err := file.Close()
		if err != nil {
			t.logError(st, err)
		}
	}()

	cmd.Stdin = file

	out, err := cmd.CombinedOutput()
	if err != nil {
		t.logError(st, fmt.Errorf("Telemetry script failed to run: %v\noutput: %s", err, out))
	}
	cmdSucceeded := err == nil

	st.Dispatch(TelemetryScriptRanAction{At: t.clock.Now()})

	// clear the file if the telemetry command succeeded
	if cmdSucceeded {
		if err = t.dir.WriteFile(tracer.OutgoingFilename, ""); err != nil {
			t.logError(st, err)
		}
	}
}

func (t *Controller) logError(st store.RStore, err error) {
	spanID := logstore.SpanID(fmt.Sprintf("telemetry:%s", string(t.runCounter)))
	st.Dispatch(configs.TiltfileLogAction{
		LogEvent: store.NewLogEvent(model.TiltfileManifestName, spanID, []byte(err.Error())),
	})
}
