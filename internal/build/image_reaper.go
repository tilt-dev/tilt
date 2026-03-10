package build

import (
	"context"
	"fmt"
	"time"

	cerrdefs "github.com/containerd/errdefs"
	"github.com/distribution/reference"
	mobyclient "github.com/moby/moby/client"
	"github.com/pkg/errors"
	"golang.org/x/sync/errgroup"

	"github.com/tilt-dev/tilt/internal/docker"
	"github.com/tilt-dev/tilt/internal/dockerfile"
)

type ImageReaper struct {
	docker docker.Client
}

func FilterByLabel(label dockerfile.Label) mobyclient.Filters {
	return make(mobyclient.Filters).Add("label", string(label))
}

func FilterByLabelValue(label dockerfile.Label, val dockerfile.LabelValue) mobyclient.Filters {
	return make(mobyclient.Filters).Add("label", fmt.Sprintf("%s=%s", label, val))
}

func FilterByRefName(ref reference.Named) mobyclient.Filters {
	return make(mobyclient.Filters).Add("reference", fmt.Sprintf("%s:*", ref.Name()))
}

func NewImageReaper(docker docker.Client) ImageReaper {
	return ImageReaper{
		docker: docker,
	}
}

// Delete all Tilt builds
//
// For safety reasons, we only delete images with the tilt.buildMode label,
// but we let the caller set additional filters.
func (r ImageReaper) RemoveTiltImages(ctx context.Context, createdBefore time.Time, force bool, extraFilters ...mobyclient.Filters) error {
	f := FilterByLabel(BuildMode)
	for _, ef := range extraFilters {
		for k, vals := range ef {
			for v := range vals {
				f.Add(k, v)
			}
		}
	}
	listOptions := mobyclient.ImageListOptions{
		Filters: f,
	}

	summaries, err := r.docker.ImageList(ctx, listOptions)
	if err != nil {
		return errors.Wrap(err, "RemoveTiltImages")
	}

	g, ctx := errgroup.WithContext(ctx)
	rmOptions := mobyclient.ImageRemoveOptions{
		PruneChildren: true,
		Force:         force,
	}
	for _, summary := range summaries {
		id := summary.ID
		created := time.Unix(summary.Created, 0)
		if !created.Before(createdBefore) {
			continue
		}

		g.Go(func() error {
			_, err := r.docker.ImageRemove(ctx, id, rmOptions)
			if cerrdefs.IsNotFound(err) {
				return nil
			}
			return err
		})
	}

	err = g.Wait()
	if err != nil {
		return errors.Wrap(err, "RemoveTiltImages")
	}
	return err
}
