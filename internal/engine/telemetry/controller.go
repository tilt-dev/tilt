package telemetry

import (
	"context"
	"fmt"
	"os/exec"
	"time"

	"github.com/windmilleng/tilt/internal/build"
	"github.com/windmilleng/tilt/internal/engine/configs"
	"github.com/windmilleng/tilt/internal/store"
	"github.com/windmilleng/tilt/internal/tracer"
	"github.com/windmilleng/tilt/pkg/model"
	"github.com/windmilleng/tilt/pkg/model/logstore"
)

type Controller struct {
	spans      tracer.SpanSource
	clock      build.Clock
	runCounter int
}

func NewController(clock build.Clock, spans tracer.SpanSource) *Controller {
	return &Controller{
		clock:      clock,
		spans:      spans,
		runCounter: 0,
	}
}

func (t *Controller) OnChange(ctx context.Context, st store.RStore) {
	state := st.RLockState()
	lastTelemetryRun := state.LastTelemetryScriptRun
	tc := model.ToShellCmd("cat >> /home/dbentley/spans.txt")
	st.RUnlockState()

	t.maybeRunScript(ctx, tc, lastTelemetryRun, st)
}

func (t *Controller) maybeRunScript(ctx context.Context, tc model.Cmd, lastTelemetryRun time.Time, st store.RStore) {
	if tc.Empty() || !lastTelemetryRun.Add(3*time.Second).Before(t.clock.Now()) {
		return
	}

	t.logError(st, fmt.Errorf("executing %v %v\n", t.clock.Now(), lastTelemetryRun))
	t.runCounter++

	// exec the telemetry command, passing in the contents of the file on stdin
	cmd := exec.CommandContext(ctx, tc.Argv[0], tc.Argv[1:]...)

	r, releaseCh, err := t.spans.GetOutgoingSpans()
	if err != nil {
		t.logError(st, fmt.Errorf("Error gathering Telemetry data for experimental_telemetry_cmd", err))
	}

	defer close(releaseCh)

	cmd.Stdin = r

	out, err := cmd.CombinedOutput()
	if err != nil {
		t.logError(st, fmt.Errorf("Telemetry script failed to run: %v\noutput: %s", err, out))
	}

	releaseCh <- true
	st.Dispatch(TelemetryScriptRanAction{At: t.clock.Now()})
}

func (t *Controller) logError(st store.RStore, err error) {
	spanID := logstore.SpanID(fmt.Sprintf("telemetry:%s", string(t.runCounter)))
	st.Dispatch(configs.TiltfileLogAction{
		LogEvent: store.NewLogEvent(model.TiltfileManifestName, spanID, []byte(err.Error())),
	})
}
