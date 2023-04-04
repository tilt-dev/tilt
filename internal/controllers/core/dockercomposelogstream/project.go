package dockercomposelogstream

import (
	"context"
	"fmt"

	"github.com/docker/docker/client"

	"github.com/tilt-dev/tilt/internal/controllers/apicmp"
	"github.com/tilt-dev/tilt/internal/dockercompose"
	"github.com/tilt-dev/tilt/pkg/apis/core/v1alpha1"
	"github.com/tilt-dev/tilt/pkg/logger"
)

// Keeps track of the projects we're currently watching.
type ProjectWatch struct {
	ctx     context.Context
	cancel  func()
	project v1alpha1.DockerComposeProject
	hash    string
}

// Sync all the project watches with the dockercompose objects
// we're currently tracking.
func (r *Reconciler) manageOwnedProjectWatches() {
	running := map[string]bool{}
	for key := range r.projectWatches {
		running[key] = true
	}

	owned := map[string]bool{}
	for _, result := range r.results {
		hash := result.projectHash
		owned[hash] = true

		if hash != "" && !running[hash] {
			ctx, cancel := context.WithCancel(result.loggerCtx)
			pw := &ProjectWatch{
				ctx:     ctx,
				cancel:  cancel,
				project: result.spec.Project,
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

			key := serviceKey{service: evt.Service, projectHash: pw.hash}
			c, err := r.getContainerInfo(ctx, evt.ID)
			if err != nil {
				if !client.IsErrNotFound(err) {
					logger.Get(ctx).Debugf("[dcwatch]: %v", err)
				}
				continue
			}

			r.mu.Lock()
			if r.recordContainerInfo(key, c) {
				r.requeueForServiceKey(key)
			}
			r.mu.Unlock()

		case <-ctx.Done():
			return
		}
	}
}

// Fetch the state of the given container and convert it into our internal model.
func (r *Reconciler) getContainerInfo(ctx context.Context, id string) (*containerInfo, error) {
	containerJSON, err := r.dc.ContainerInspect(ctx, id)
	if err != nil {
		return nil, err
	}

	if containerJSON.Config == nil ||
		containerJSON.ContainerJSONBase == nil ||
		containerJSON.ContainerJSONBase.State == nil {
		return nil, fmt.Errorf("no state found")
	}

	cState := containerJSON.ContainerJSONBase.State
	return &containerInfo{
		id:    id,
		state: dockercompose.ToContainerState(cState),
		tty:   containerJSON.Config.Tty,
	}, nil
}

// Record the container event and re-reconcile. Caller must hold the lock.
// Returns true on change.
func (r *Reconciler) recordContainerInfo(key serviceKey, c *containerInfo) bool {
	existing := r.containers[key]
	if apicmp.DeepEqual(c, existing) {
		return false
	}

	r.containers[key] = c
	return true
}

// Find any results that depend on the given service, and ask the
// reconciler to re-concile.
func (r *Reconciler) requeueForServiceKey(key serviceKey) {
	for _, result := range r.results {
		if result.serviceKey() == key {
			r.requeuer.Add(result.name)
		}
	}
}
