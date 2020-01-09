package local

import (
	"context"
	"fmt"
	"time"

	"github.com/windmilleng/tilt/internal/store"
	"github.com/windmilleng/tilt/pkg/logger"
	"github.com/windmilleng/tilt/pkg/model"
	"github.com/windmilleng/tilt/pkg/model/logstore"
)

type Controller struct {
	execer Execer
	procs  map[model.ManifestName]*currentProcess
}

func NewController(execer Execer) *Controller {
	return &Controller{
		execer: execer,
		procs:  make(map[model.ManifestName]*currentProcess),
	}
}

func (c *Controller) OnChange(ctx context.Context, st store.RStore) {
	specs := c.determineServeSpecs(ctx, st)
	c.update(ctx, specs, st)
}

func (c *Controller) determineServeSpecs(ctx context.Context, st store.RStore) []ServeSpec {
	state := st.RLockState()
	defer st.RUnlockState()

	var r []ServeSpec

	for _, mt := range state.Targets() {
		if !mt.Manifest.IsLocal() {
			continue
		}
		lt := mt.Manifest.LocalTarget()
		if lt.ServeCmd.Empty() ||
			mt.State.LastSuccessfulDeployTime.IsZero() {
			continue
		}
		r = append(r, ServeSpec{
			mt.Manifest.Name,
			lt.ServeCmd,
			mt.State.LastSuccessfulDeployTime,
		})
	}

	return r
}

func (c *Controller) update(ctx context.Context, specs []ServeSpec, st store.RStore) {
	var toStop []model.ManifestName
	var toStart []ServeSpec

	seen := make(map[model.ManifestName]bool)
	for _, spec := range specs {
		seen[spec.ManifestName] = true
		proc := c.getOrMakeProc(spec.ManifestName)
		if c.shouldStart(spec, proc) {
			toStart = append(toStart, spec)
		}
	}

	for name, _ := range c.procs {
		if !seen[name] {
			toStop = append(toStop, name)
		}
	}

	// stop old ones
	for _, name := range toStop {
		c.stop(name)
		delete(c.procs, name)
	}

	// now start them
	for _, spec := range toStart {
		c.start(ctx, spec, st)
	}
}

func (c *Controller) shouldStart(spec ServeSpec, proc *currentProcess) bool {
	if proc.cancelFunc == nil {
		// nothing is running, so start it
		return true
	}

	if spec.TriggerTime.After(proc.spec.TriggerTime) {
		// there's been a new trigger, so start it
		return true
	}

	return false
}

func (c *Controller) stop(name model.ManifestName) {
	proc := c.procs[name]
	if proc.stoppedCh != nil {
		close(proc.stoppedCh)
	}
	if proc.cancelFunc == nil {
		return
	}
	proc.cancelFunc()
	<-proc.doneCh
	proc.cancelFunc = nil
	proc.doneCh = nil
}

func (c *Controller) getOrMakeProc(name model.ManifestName) *currentProcess {
	if c.procs[name] == nil {
		c.procs[name] = &currentProcess{}
	}

	return c.procs[name]
}

func (c *Controller) start(ctx context.Context, spec ServeSpec, st store.RStore) {
	c.stop(spec.ManifestName)

	proc := c.procs[spec.ManifestName]
	proc.spec = spec
	proc.stoppedCh = make(chan struct{})
	proc.sequenceNum++
	ctx, proc.cancelFunc = context.WithCancel(ctx)

	w := LocalServeLogActionWriter{
		store:        st,
		manifestName: spec.ManifestName,
		sequenceNum:  proc.sequenceNum,
	}
	ctx = logger.CtxWithLogHandler(ctx, w)

	statusCh := make(chan status)

	go processStatuses(statusCh, proc.stoppedCh, st, spec.ManifestName, proc.sequenceNum)

	proc.doneCh = c.execer.Start(ctx, spec.ServeCmd, logger.Get(ctx).Writer(logger.InfoLvl), statusCh)
}

func processStatuses(
	statusCh chan status,
	stoppedCh chan struct{},
	st store.RStore,
	manifestName model.ManifestName,
	sequenceNum int) {
	stopped := false
	for {
		select {
		case status, ok := <-statusCh:
			if !ok {
				return
			}
			if stopped {
				continue
			}
			runtimeStatus := status.ToRuntime()
			if runtimeStatus != "" {
				// TODO(matt) when we get an error, the dot is red in the web ui, but green in the TUI
				st.Dispatch(LocalServeStatusAction{
					ManifestName: manifestName,
					SequenceNum:  sequenceNum,
					Status:       runtimeStatus,
				})
			}
		case <-stoppedCh:
			// if we stopped the process ourselves, we don't need to report any more status updates
			stopped = true
		}
	}
}

// currentProcess represents the current process for a Manifest, so that Controller can
// make sure there's at most one process per Manifest.
// (note: it may not be running yet, or may have already finished)
type currentProcess struct {
	spec        ServeSpec
	sequenceNum int
	cancelFunc  context.CancelFunc
	// closed when the process finishes executing, intentionally or not
	doneCh chan struct{}
	// closed when the controller intentionally stopped the process
	stoppedCh chan struct{}
}

type LocalServeLogActionWriter struct {
	store        store.RStore
	manifestName model.ManifestName
	sequenceNum  int
}

func (w LocalServeLogActionWriter) Write(level logger.Level, p []byte) error {
	w.store.Dispatch(store.NewLogEvent(w.manifestName, SpanIDForServeLog(w.sequenceNum), level, p))
	return nil
}

func SpanIDForServeLog(sequenceNum int) logstore.SpanID {
	return logstore.SpanID(fmt.Sprintf("localserve:%d", sequenceNum))
}

// ServeSpec describes what Runner should be running
type ServeSpec struct {
	ManifestName model.ManifestName
	ServeCmd     model.Cmd
	TriggerTime  time.Time // TriggerTime is how Runner knows to restart; if it's newer than the TriggerTime of the currently running command, then Runner should restart it
}

type status int

const (
	Unknown status = iota
	Running status = iota
	Done    status = iota
	Error   status = iota
)

func (s status) ToRuntime() model.RuntimeStatus {
	switch s {
	case Running:
		return model.RuntimeStatusOK
	case Error:
		return model.RuntimeStatusError
	default:
		return ""
	}
}
