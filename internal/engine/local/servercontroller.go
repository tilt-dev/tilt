package local

import (
	"context"
	"fmt"
	"time"

	"k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/tilt-dev/tilt/internal/store"
	"github.com/tilt-dev/tilt/pkg/apis/core/v1alpha1"
	"github.com/tilt-dev/tilt/pkg/model"
	"github.com/tilt-dev/tilt/pkg/model/logstore"
)

const AnnotationOwnerName = "tilt.dev/owner-name"
const AnnotationOwnerKind = "tilt.dev/owner-kind"

// Expresses the status of a build dependency.
const AnnotationDepStatus = "tilt.dev/dep-status"

// A controller that reads the Tilt data model and creates new Cmd objects.
//
// Reads the Cmd Status.
//
// A CmdServer offers two constraints on top of a Cmd:
//
// - We ensure that the old Cmd is terminated before we replace it
//   with a new one, because they likely use the same port.
//
// - We report the Cmd status Terminated as an Error state,
//   and report it in a standard way.
type ServerController struct {
	recentlyCreatedCmd map[string]string
	createdTriggerTime map[string]time.Time
	client             ctrlclient.Client

	cmdCount int
}

var _ store.Subscriber = &ServerController{}

func NewServerController(client ctrlclient.Client) *ServerController {
	return &ServerController{
		recentlyCreatedCmd: make(map[string]string),
		createdTriggerTime: make(map[string]time.Time),
		client:             client,
	}
}

func (c *ServerController) OnChange(ctx context.Context, st store.RStore, summary store.ChangeSummary) {
	if summary.IsLogOnly() {
		return
	}

	servers, owned, orphans := c.determineServers(ctx, st)
	for i, server := range servers {
		c.reconcile(ctx, server, owned[i], st)
	}

	// Garbage collect commands where the owner has been deleted.
	for _, orphan := range orphans {
		c.deleteOrphanedCmd(ctx, st, orphan)
	}
}

// Returns a list of server objects and the Cmd they own (if any).
func (c *ServerController) determineServers(ctx context.Context, st store.RStore) (servers []CmdServer, owned [][]*Cmd, orphaned []*Cmd) {
	state := st.RLockState()
	defer st.RUnlockState()

	// Find all the Cmds owned by CmdServer.
	//
	// Simulates controller-runtime's notion of Owns().
	ownedCmds := make(map[string][]*Cmd)
	for _, cmd := range state.Cmds {
		ownerName := cmd.Annotations[AnnotationOwnerName]
		ownerKind := cmd.Annotations[AnnotationOwnerKind]
		if ownerKind != "CmdServer" {
			continue
		}

		ownedCmds[ownerName] = append(ownedCmds[ownerName], cmd)
	}

	// Infer all the CmdServer objects from the legacy EngineState
	for _, mt := range state.Targets() {
		if !mt.Manifest.IsLocal() {
			continue
		}
		lt := mt.Manifest.LocalTarget()
		if lt.ServeCmd.Empty() {
			continue
		}

		name := mt.Manifest.Name.String()
		cmdServer := CmdServer{
			ObjectMeta: ObjectMeta{
				Name: name,
				Annotations: map[string]string{
					AnnotationDepStatus: string(mt.UpdateStatus()),
				},
			},
			Spec: CmdServerSpec{
				Args:           lt.ServeCmd.Argv,
				Dir:            lt.ServeCmd.Dir,
				Env:            lt.ServeCmd.Env,
				TriggerTime:    mt.State.LastSuccessfulDeployTime,
				ReadinessProbe: lt.ReadinessProbe,
			},
		}

		mn := mt.Manifest.Name.String()
		cmds, ok := ownedCmds[mn]
		if ok {
			delete(ownedCmds, mn)
		}

		servers = append(servers, cmdServer)
		owned = append(owned, cmds)
	}

	for _, orphan := range ownedCmds {
		orphaned = append(orphaned, orphan...)
	}

	return servers, owned, orphaned
}

// Find the most recent command in a collection
func (c *ServerController) mostRecentCmd(cmds []*Cmd) *Cmd {
	var mostRecentCmd *Cmd
	for _, cmd := range cmds {
		if mostRecentCmd == nil {
			mostRecentCmd = cmd
			continue
		}

		if cmd.CreationTimestamp.Time.Equal(mostRecentCmd.CreationTimestamp.Time) {
			if cmd.Name > mostRecentCmd.Name {
				mostRecentCmd = cmd
			}
			continue
		}

		if cmd.CreationTimestamp.Time.After(mostRecentCmd.CreationTimestamp.Time) {
			mostRecentCmd = cmd
		}
	}

	return mostRecentCmd
}

// Delete a command and stop waiting on it.
func (c *ServerController) deleteOwnedCmd(ctx context.Context, serverName string, st store.RStore, cmd *Cmd) {
	if waitingOn := c.recentlyCreatedCmd[serverName]; waitingOn == cmd.Name {
		delete(c.recentlyCreatedCmd, serverName)
	}

	c.deleteOrphanedCmd(ctx, st, cmd)
}

// Delete an orphaned command.
func (c *ServerController) deleteOrphanedCmd(ctx context.Context, st store.RStore, cmd *Cmd) {
	err := c.client.Delete(ctx, cmd)

	// We want our reconciler to be idempotent, so it's OK
	// if it deletes the same resource multiple times
	if err != nil && !apierrors.IsNotFound(err) {
		st.Dispatch(store.NewErrorAction(fmt.Errorf("deleting Cmd from apiserver: %v", err)))
		return
	}

	st.Dispatch(CmdDeleteAction{Name: cmd.Name})
}

func (c *ServerController) reconcile(ctx context.Context, server CmdServer, ownedCmds []*Cmd, st store.RStore) {
	// Do not make any changes to the server while the update status is building.
	// This ensures the old server stays up while any deps are building.
	depStatus := model.UpdateStatus(server.ObjectMeta.Annotations[AnnotationDepStatus])
	if depStatus != model.UpdateStatusOK && depStatus != model.UpdateStatusNotApplicable {
		return
	}

	name := server.Name

	// If the command was created recently but hasn't appeared yet, wait until it appears.
	if waitingOn, ok := c.recentlyCreatedCmd[name]; ok {
		seen := false
		for _, owned := range ownedCmds {
			if waitingOn == owned.Name {
				seen = true
			}
		}

		if !seen {
			return
		}
		delete(c.recentlyCreatedCmd, name)
	}

	cmdSpec := CmdSpec{
		Args:           server.Spec.Args,
		Dir:            server.Spec.Dir,
		Env:            server.Spec.Env,
		ReadinessProbe: server.Spec.ReadinessProbe,
	}

	triggerTime := c.createdTriggerTime[name]
	mostRecent := c.mostRecentCmd(ownedCmds)
	if mostRecent != nil && equality.Semantic.DeepEqual(mostRecent.Spec, cmdSpec) && triggerTime.Equal(server.Spec.TriggerTime) {
		// We're in the correct state! Nothing to do.
		return
	}

	// Otherwise, we need to create a new command.

	// Garbage collect all owned commands.
	for _, owned := range ownedCmds {
		c.deleteOwnedCmd(ctx, name, st, owned)
	}

	// If any commands are still running, we need to wait.
	// Otherwise, we'll run into problems where a new server will
	// start while the old server is hanging onto the port.
	for _, owned := range ownedCmds {
		if owned.Status.Terminated == nil {
			return
		}
	}

	// Start the command!
	c.createdTriggerTime[name] = server.Spec.TriggerTime
	c.cmdCount++

	cmdName := fmt.Sprintf("%s-serve-%d", name, c.cmdCount)
	spanID := SpanIDForServeLog(c.cmdCount)

	cmd := &Cmd{
		ObjectMeta: ObjectMeta{
			Name: cmdName,
			Annotations: map[string]string{
				// TODO(nick): This should be an owner reference once CmdServer is a
				// full-fledged type.
				AnnotationOwnerName: name,
				AnnotationOwnerKind: "CmdServer",

				v1alpha1.AnnotationManifest: name,
				v1alpha1.AnnotationSpanID:   string(spanID),
			},
		},
		Spec: cmdSpec,
	}
	c.recentlyCreatedCmd[name] = cmdName

	err := c.client.Create(ctx, cmd)
	if err != nil && !apierrors.IsNotFound(err) {
		st.Dispatch(store.NewErrorAction(fmt.Errorf("syncing to apiserver: %v", err)))
		return
	}

	st.Dispatch(CmdCreateAction{Cmd: cmd})
}

type CmdServer struct {
	metav1.ObjectMeta

	Spec   CmdServerSpec
	Status CmdServerStatus
}

type CmdServerSpec struct {
	Args           []string
	Dir            string
	Env            []string
	ReadinessProbe *v1alpha1.Probe

	// Kubernetes tends to represent this as a "generation" field
	// to force an update.
	TriggerTime time.Time
}

type CmdServerStatus struct {
}

func SpanIDForServeLog(procNum int) logstore.SpanID {
	return logstore.SpanID(fmt.Sprintf("localserve:%d", procNum))
}
