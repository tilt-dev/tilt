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

var period = 60 * time.Second

func (t *Controller) OnChange(ctx context.Context, st store.RStore) {
	state := st.RLockState()
	status := state.TelemetryStatus
	tc := model.ToShellCmd("cat >> /home/dbentley/spans.txt")
	st.RUnlockState()

	if tc.Empty() ||
		status.ControllerActionCount < t.runCounter ||
		!status.LastRunAt.Add(period).Before(t.clock.Now()) {
		return
	}

	t.runCounter++

	defer func() {
		// wrap in a func so we get the time at the end of this function, not the beginning
		st.Dispatch(TelemetryScriptRanAction{
			Status: model.TelemetryStatus{
				LastRunAt:             t.clock.Now(),
				ControllerActionCount: t.runCounter,
			}})
	}()

	r, releaseCh, err := t.spans.GetOutgoingSpans()
	if err != nil {
		t.logError(st, fmt.Errorf("Error gathering Telemetry data for experimental_telemetry_cmd", err))
	}

	consumed := false

	// run the command with the contents of the spans as jsonlines on stdin
	cmd := exec.CommandContext(ctx, tc.Argv[0], tc.Argv[1:]...)
	cmd.Stdin = r

	out, err := cmd.CombinedOutput()
	if err != nil {
		t.logError(st, fmt.Errorf("Telemetry command failed: %v\noutput: %s", err, out))
		releaseCh <- false
		return
	}

	// tell the SpanSource to delete those spans
	releaseCh <- true
}

func (t *Controller) logError(st store.RStore, err error) {
	spanID := logstore.SpanID(fmt.Sprintf("telemetry:%s", string(t.runCounter)))
	st.Dispatch(configs.TiltfileLogAction{
		LogEvent: store.NewLogEvent(model.TiltfileManifestName, spanID, []byte(err.Error())),
	})
}
