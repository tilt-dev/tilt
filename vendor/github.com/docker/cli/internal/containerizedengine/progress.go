package containerizedengine

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"sync"
	"time"

	"github.com/containerd/containerd/content"
	"github.com/containerd/containerd/errdefs"
	"github.com/containerd/containerd/remotes"
	"github.com/docker/docker/pkg/jsonmessage"
	digest "github.com/opencontainers/go-digest"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/sirupsen/logrus"
)

func showProgress(ctx context.Context, ongoing *jobs, cs content.Store, out io.WriteCloser) {
	var (
		ticker   = time.NewTicker(100 * time.Millisecond)
		start    = time.Now()
		enc      = json.NewEncoder(out)
		statuses = map[string]statusInfo{}
		done     bool
	)
	defer ticker.Stop()

outer:
	for {
		select {
		case <-ticker.C:

			resolved := "resolved"
			if !ongoing.isResolved() {
				resolved = "resolving"
			}
			statuses[ongoing.name] = statusInfo{
				Ref:    ongoing.name,
				Status: resolved,
			}
			keys := []string{ongoing.name}

			activeSeen := map[string]struct{}{}
			if !done {
				active, err := cs.ListStatuses(ctx, "")
				if err != nil {
					logrus.Debugf("active check failed: %s", err)
					continue
				}
				// update status of active entries!
				for _, active := range active {
					statuses[active.Ref] = statusInfo{
						Ref:       active.Ref,
						Status:    "downloading",
						Offset:    active.Offset,
						Total:     active.Total,
						StartedAt: active.StartedAt,
						UpdatedAt: active.UpdatedAt,
					}
					activeSeen[active.Ref] = struct{}{}
				}
			}

			err := updateNonActive(ctx, ongoing, cs, statuses, keys, activeSeen, &done, start)
			if err != nil {
				continue outer
			}

			var ordered []statusInfo
			for _, key := range keys {
				ordered = append(ordered, statuses[key])
			}

			for _, si := range ordered {
				jm := si.JSONMessage()
				err := enc.Encode(jm)
				if err != nil {
					logrus.Debugf("failed to encode progress message: %s", err)
				}
			}

			if done {
				out.Close()
				return
			}
		case <-ctx.Done():
			done = true // allow ui to update once more
		}
	}
}

func updateNonActive(ctx context.Context, ongoing *jobs, cs content.Store, statuses map[string]statusInfo, keys []string, activeSeen map[string]struct{}, done *bool, start time.Time) error {

	for _, j := range ongoing.jobs() {
		key := remotes.MakeRefKey(ctx, j)
		keys = append(keys, key)
		if _, ok := activeSeen[key]; ok {
			continue
		}

		status, ok := statuses[key]
		if !*done && (!ok || status.Status == "downloading") {
			info, err := cs.Info(ctx, j.Digest)
			if err != nil {
				if !errdefs.IsNotFound(err) {
					logrus.Debugf("failed to get content info: %s", err)
					return err
				}
				statuses[key] = statusInfo{
					Ref:    key,
					Status: "waiting",
				}
			} else if info.CreatedAt.After(start) {
				statuses[key] = statusInfo{
					Ref:       key,
					Status:    "done",
					Offset:    info.Size,
					Total:     info.Size,
					UpdatedAt: info.CreatedAt,
				}
			} else {
				statuses[key] = statusInfo{
					Ref:    key,
					Status: "exists",
				}
			}
		} else if *done {
			if ok {
				if status.Status != "done" && status.Status != "exists" {
					status.Status = "done"
					statuses[key] = status
				}
			} else {
				statuses[key] = statusInfo{
					Ref:    key,
					Status: "done",
				}
			}
		}
	}
	return nil
}

type jobs struct {
	name     string
	added    map[digest.Digest]struct{}
	descs    []ocispec.Descriptor
	mu       sync.Mutex
	resolved bool
}

func newJobs(name string) *jobs {
	return &jobs{
		name:  name,
		added: map[digest.Digest]struct{}{},
	}
}

func (j *jobs) add(desc ocispec.Descriptor) {
	j.mu.Lock()
	defer j.mu.Unlock()
	j.resolved = true

	if _, ok := j.added[desc.Digest]; ok {
		return
	}
	j.descs = append(j.descs, desc)
	j.added[desc.Digest] = struct{}{}
}

func (j *jobs) jobs() []ocispec.Descriptor {
	j.mu.Lock()
	defer j.mu.Unlock()

	var descs []ocispec.Descriptor
	return append(descs, j.descs...)
}

func (j *jobs) isResolved() bool {
	j.mu.Lock()
	defer j.mu.Unlock()
	return j.resolved
}

// statusInfo holds the status info for an upload or download
type statusInfo struct {
	Ref       string
	Status    string
	Offset    int64
	Total     int64
	StartedAt time.Time
	UpdatedAt time.Time
}

func (s statusInfo) JSONMessage() jsonmessage.JSONMessage {
	// Shorten the ID to use up less width on the display
	id := s.Ref
	if strings.Contains(id, ":") {
		split := strings.SplitN(id, ":", 2)
		id = split[1]
	}
	id = fmt.Sprintf("%.12s", id)

	return jsonmessage.JSONMessage{
		ID:     id,
		Status: s.Status,
		Progress: &jsonmessage.JSONProgress{
			Current: s.Offset,
			Total:   s.Total,
		},
	}
}
