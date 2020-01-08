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
	procs  map[model.ManifestName]*lastProcess
}

func NewController(execer Execer) *Controller {
	return &Controller{
		execer: execer,
		procs:  make(map[model.ManifestName]*lastProcess),
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
	}

	// now start them
	for _, spec := range toStart {
		c.start(ctx, spec, st)
	}
}

func (c *Controller) shouldStart(spec ServeSpec, proc *lastProcess) bool {
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
	if proc.cancelFunc == nil {
		return
	}
	proc.cancelFunc()
	<-proc.doneCh
	proc.cancelFunc = nil
	proc.doneCh = nil
}

func (c *Controller) getOrMakeProc(name model.ManifestName) *lastProcess {
	if c.procs[name] == nil {
		c.procs[name] = &lastProcess{}
	}

	return c.procs[name]
}

func (c *Controller) start(ctx context.Context, spec ServeSpec, st store.RStore) {
	c.stop(spec.ManifestName)

	proc := c.procs[spec.ManifestName]
	proc.sequenceNum++
	ctx, cancelFunc := context.WithCancel(ctx)
	proc.cancelFunc = cancelFunc

	w := LocalServeLogActionWriter{
		store:        st,
		manifestName: spec.ManifestName,
		sequenceNum:  proc.sequenceNum,
	}
	ctx = logger.CtxWithLogHandler(ctx, w)

	statusCh := make(chan Status)
	sequenceNum := proc.sequenceNum
	go func() {
		for status := range statusCh {
			st.Dispatch(LocalServeStatusAction{
				ManifestName: spec.ManifestName,
				SequenceNum:  sequenceNum,
				Status:       status,
			})
		}
	}()

	proc.doneCh = c.execer.Start(ctx, spec.ServeCmd, logger.Get(ctx).Writer(logger.InfoLvl), statusCh)
}

// lastProcess represents the last process
type lastProcess struct {
	spec        ServeSpec
	sequenceNum int
	cancelFunc  context.CancelFunc
	doneCh      chan struct{}
}

type LocalServeLogActionWriter struct {
	store        store.RStore
	manifestName model.ManifestName
	sequenceNum  int
}

func (w LocalServeLogActionWriter) Write(level logger.Level, p []byte) error {
	w.store.Dispatch(LocalServeLogAction{
		LogEvent: store.NewLogEvent(w.manifestName, SpanIDForServeLog(w.sequenceNum), level, p),
	})
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

type Status int

const (
	Unknown Status = iota
	Running
	Done
	Error
)
