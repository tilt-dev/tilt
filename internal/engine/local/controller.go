package local

import (
	"context"
	"fmt"
	"log"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/tilt-dev/tilt/internal/store"
	"github.com/tilt-dev/tilt/pkg/apis/core/v1alpha1"
	tiltapi "github.com/tilt-dev/tilt/pkg/clientset/tiltapi/typed/core/v1alpha1"
	"github.com/tilt-dev/tilt/pkg/logger"
	"github.com/tilt-dev/tilt/pkg/model"
)

type CmdInterface = tiltapi.CmdInterface
type Cmd = v1alpha1.Cmd
type CmdSpec = v1alpha1.CmdSpec

const LabelManifest = v1alpha1.LabelManifest

type proc struct {
	spec ServeSpec
	name string
}

type Controller struct {
	cmdClient CmdInterface
	procs     map[model.ManifestName]*proc
	procCount int
}

var _ store.Subscriber = &Controller{}

func NewController(cmdClient CmdInterface) *Controller {
	return &Controller{
		cmdClient: cmdClient,
		procs:     make(map[model.ManifestName]*proc),
		procCount: 0,
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
			ManifestName:   mt.Manifest.Name,
			ServeCmd:       lt.ServeCmd,
			TriggerTime:    mt.State.LastSuccessfulDeployTime,
			ReadinessProbe: lt.ReadinessProbe,
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
		proc := c.procs[spec.ManifestName]
		if c.shouldStart(spec, proc) {
			toStart = append(toStart, spec)
		}
	}

	for name := range c.procs {
		if !seen[name] {
			toStop = append(toStop, name)
		}
	}

	// stop old ones
	for _, name := range toStop {
		c.stop(ctx, name)
	}

	// now start them
	for _, spec := range toStart {
		c.start(ctx, spec, st)
	}
}

func (c *Controller) shouldStart(spec ServeSpec, proc *proc) bool {
	if proc == nil {
		// nothing is running, so start it
		return true
	}

	if spec.TriggerTime.After(proc.spec.TriggerTime) {
		// there's been a new trigger, so start it
		return true
	}

	return false
}

func (c *Controller) stop(ctx context.Context, name model.ManifestName) {
	proc, ok := c.procs[name]
	if !ok {
		return
	}
	delete(c.procs, name)

	err := c.cmdClient.Delete(ctx, proc.name, metav1.DeleteOptions{})
	if err != nil {
		logger.Get(ctx).Debugf("delete cmd: %v", err)
	}
}

func (c *Controller) start(ctx context.Context, spec ServeSpec, st store.RStore) {
	c.stop(ctx, spec.ManifestName)

	c.procCount++
	name := fmt.Sprintf("%s-serve-%d", spec.ManifestName, c.procCount)
	c.procs[spec.ManifestName] = &proc{
		spec: spec,
		name: name,
	}

	cmd := &Cmd{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
			Labels: map[string]string{
				LabelManifest: spec.ManifestName.String(),
			},
		},
		Spec: CmdSpec{
			Args:           spec.ServeCmd.Argv,
			Dir:            spec.ServeCmd.Dir,
			Env:            spec.ServeCmd.Env,
			ReadinessProbe: spec.ReadinessProbe,
		},
	}

	log.Println("CREATING COMMAND", cmd)
	_, err := c.cmdClient.Create(ctx, cmd, metav1.CreateOptions{})
	if err != nil {
		logger.Get(ctx).Debugf("create: %v", err)
		return
	}
	st.Dispatch(CmdCreateAction{
		ManifestName: spec.ManifestName,
		Cmd:          cmd,
	})
}

// ServeSpec describes what Runner should be running
type ServeSpec struct {
	ManifestName   model.ManifestName
	ServeCmd       model.Cmd
	TriggerTime    time.Time // TriggerTime is how Runner knows to restart; if it's newer than the TriggerTime of the currently running command, then Runner should restart it
	ReadinessProbe *v1alpha1.Probe
}
