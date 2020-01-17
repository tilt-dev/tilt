package telemetry

import (
	"context"
	"fmt"
	"io"
	"os/exec"
	"time"

	"github.com/windmilleng/tilt/internal/build"
	"github.com/windmilleng/tilt/internal/store"
	"github.com/windmilleng/tilt/internal/tracer"
	"github.com/windmilleng/tilt/pkg/logger"
	"github.com/windmilleng/tilt/pkg/model"
	"github.com/windmilleng/tilt/pkg/model/logstore"
)

type Controller struct {
	spans      tracer.SpanSource
	clock      build.Clock
	runCounter int
	lastRunAt  time.Time
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
	ts := state.TelemetrySettings
	tc := ts.Cmd
	st.RUnlockState()

	if tc.Empty() || !t.lastRunAt.Add(period).Before(t.clock.Now()) {
		return
	}

	t.runCounter++

	defer func() {
		// wrap in a func so we get the time at the end of this function, not the beginning
		t.lastRunAt = t.clock.Now()
	}()

	r, requeueFn, err := t.spans.GetOutgoingSpans()
	if err != nil {
		if err != io.EOF {
			t.logError(st, fmt.Errorf("Error gathering Telemetry data for experimental_telemetry_cmd %v", err))
		}
		return
	}

	// run the command with the contents of the spans as jsonlines on stdin
	cmd := exec.CommandContext(ctx, tc.Argv[0], tc.Argv[1:]...)
	cmd.Dir = ts.Workdir
	cmd.Stdin = r

	out, err := cmd.CombinedOutput()
	if err != nil {
		t.logError(st, fmt.Errorf("Telemetry command failed: %v\noutput: %s", err, out))
		requeueFn()
		return
	}
}

func (t *Controller) logError(st store.RStore, err error) {
	spanID := logstore.SpanID(fmt.Sprintf("telemetry:%s", string(t.runCounter)))
	st.Dispatch(store.NewLogAction(model.TiltfileManifestName, spanID, logger.InfoLvl, nil, []byte(err.Error())))
}
