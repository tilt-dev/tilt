package dockercomposeservice

import (
	"context"

	"github.com/tilt-dev/tilt/internal/controllers/apicmp"
	"github.com/tilt-dev/tilt/internal/dockercompose"
	"github.com/tilt-dev/tilt/pkg/apis/core/v1alpha1"
	"github.com/tilt-dev/tilt/pkg/logger"
)

// Sync all the project watches with the dockercompose objects
// we're currently tracking.
func (r *Reconciler) manageOwnedProjectWatches(ctx context.Context) {
	r.mu.Lock()
	defer r.mu.Unlock()

	running := map[string]bool{}
	for key := range r.projectWatches {
		running[key] = true
	}

	owned := map[string]bool{}
	for _, result := range r.results {
		hash := result.ProjectHash
		owned[hash] = true

		if hash != "" && !running[hash] {
			ctx, cancel := context.WithCancel(ctx)
			pw := &ProjectWatch{
				ctx:     ctx,
				cancel:  cancel,
				project: result.Spec.Project,
				hash:    hash,
			}
			r.projectWatches[hash] = pw
			go r.runProjectWatch(pw)
			running[hash] = true
		}
	}

	for key := range r.projectWatches {
		if !owned[key] {
			r.projectWatches[key].cancel()
			delete(r.projectWatches, key)
		}
	}
}

// Stream events from the docker-compose project and
// fan them out to each service in the project.
func (r *Reconciler) runProjectWatch(pw *ProjectWatch) {
	defer func() {
		r.mu.Lock()
		delete(r.projectWatches, pw.hash)
		r.mu.Unlock()
		pw.cancel()
	}()

	ctx := pw.ctx
	project := pw.project
	ch, err := r.dcc.StreamEvents(ctx, project)
	if err != nil {
		// TODO(nick): Figure out where this error should be published.
		return
	}

	for {
		select {
		case evtJson, ok := <-ch:
			if !ok {
				return
			}
			evt, err := dockercompose.EventFromJsonStr(evtJson)
			if err != nil {
				logger.Get(ctx).Debugf("[dcwatch] failed to unmarshal dc event '%s' with err: %v", evtJson, err)
				continue
			}

			if evt.Type != dockercompose.TypeContainer {
				continue
			}

			containerJSON, err := r.dc.ContainerInspect(ctx, evt.ID)
			if err != nil {
				logger.Get(ctx).Debugf("[dcwatch] inspecting container: %v", err)
				continue
			}

			if containerJSON.ContainerJSONBase == nil || containerJSON.ContainerJSONBase.State == nil {
				logger.Get(ctx).Debugf("[dcwatch] inspecting container: no state found")
				continue
			}

			cState := containerJSON.ContainerJSONBase.State
			dcState := dockercompose.ToContainerState(cState)
			r.recordContainerEvent(evt, dcState)

		case <-ctx.Done():
			return
		}
	}
}

// Record the container event and re-reconcile the dockercompose service.
func (r *Reconciler) recordContainerEvent(evt dockercompose.Event, state *v1alpha1.DockerContainerState) {
	r.mu.Lock()
	defer r.mu.Unlock()

	result, ok := r.resultsByServiceName[evt.Service]
	if !ok {
		return
	}

	if apicmp.DeepEqual(state, result.Status.ContainerState) {
		return
	}

	// No need to copy because this is a value struct.
	update := result.Status
	update.ContainerID = evt.ID
	update.ContainerState = state
	result.Status = update
	r.requeuer.Add(result.Name)
}
